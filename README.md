# credential-1password

A credential helper which stores secrets in 1Password and interfaces seamlessly with both git and docker. 1Password issues session tokens which remain valid until unused for 30min -> development flows naturally since your master password is only requested for git / docker operations after periods of inactivity.

## Install
credential-1password relies on 1Password's `op` CLI under the hood to manage credentials, first follow the steps to [set up + sign in with op](https://support.1password.com/command-line-getting-started).

### Install for git
```sh
# pull binary and store in /usr/local/bin
wget https://github.com/tlowerison/credential-1password/releases/download/v1.0.1/git-credential-1password -q -O /usr/local/bin/git-credential-1password

# give executable permission
chmod u+x /usr/local/bin/git-credential-1password

# reload PATH
source ~/.bash_profile

# unset existing credential.helper in any file (can be an issue when installed with brew, osxkeychain is set by default)
# - you may need to run this more than once if multiple files have set credential.helper
git config -f $(git config --show-origin --get credential.helper | sed 's/file://' | sed 's/\t.*//') --unset credential.helper

# set as global credential store
git config --global credential.helper 1password

# Optional: set the name of the vault you want to store credentials in. Default: git-credential
# git-credential-1password vault <vault-name>

# store your credentials using key=value pairs passed into stdin
# - stdin then opens, will closes after receiving two newlines
# - after stdin closes, you'll be asked to sign into 1Password if it's been 30 minutes since you last accessed 1Password with git-credential-1password
# Ex: git-credential-1password store
#   > protocol=https
#   > host=github.com
#   > username=my-username
#   > password=my-password
#   >
#   > Enter the password for <my-1password@email.com> at my.1password.com: [type master password here]
# note: you probably want to use a Github Personal Access Token here instead of your actual password
git-credential-1password store

# confirm that your credentials are stored and retrievable
printf $'protocol=https\nhost=github.com\n' | git-credential-1password get
# > protocol=https
# > host=github.com
# > username=my-username
# > password=my-password

# finally, clone a private repo connected to the credentials you stored
# git clone https://github.com/my-username/my-repo.git
```

### Install for Docker
Update your docker version to at least `20.10.4`, there was a bug fix included that fixed docker from segfaulting when using custom credential helpers ([relevant pr](https://github.com/docker/cli/pull/2959)).
```sh
# logout of docker to remove old credentials
docker logout

# pull binary and store in /usr/local/bin
wget https://github.com/tlowerison/credential-1password/releases/download/v1.0.1/docker-credential-1password -q -O /usr/local/bin/docker-credential-1password

# give executable permission
chmod u+x /usr/local/bin/docker-credential-1password

# reload PATH
source ~/.bash_profile

# update credsStore in docker config
jq --argjson credsStore '"1password"' 'setpath(["credsStore"]; $credsStore)' ~/.docker/config.json > ~/.docker/.tmp.json && mv ~/.docker/.tmp.json ~/.docker/config.json

# Optional: set the name of the vault you want to store credentials in. Default: docker-credential
# docker-credential-1password vault <vault-name>

# login into your docker registry
# NOTE: As of now, it's essential to use the '--username' flag instead of providing as part of stdin.
# - Bug report out at https://github.com/docker/cli/issues/3022
# - Read more about the docker login command at https://docs.docker.com/engine/reference/commandline/login
docker login --username=<my-username>
# > Password: [type Personal Access Token here]
# > Enter the password for <my-1password@email.com> at my.1password.com: [type master password here]

# confirm that your credentials are stored and retrievable
printf 'https://index.docker.io/v1/' | docker-credential-1password get
# > {"ServerURL":"https://index.docker.io/v1/","Username":"my-username","Secret":"my-secret"}

# finally, pull a private image from your logged-in registry
docker pull repo/image:tag
```

## Use git credentials in docker builds

Combining `git-credential-1password` and [Docker BuildKit secrets](https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information) allows us to safely inject git credentials into containers at build time.

The idea is to wrap `docker build` with an alias `docker-build` which will:
- pull your git credentials with `git-credential-1password get`
- format those credentials into url format for use with the basic `store` git credential helper in our Dockerfile
- call `docker build` with (a) all the same arguments provided to `docker-build` and (b) our git credentials

In order to use the git credentials in your Dockerfile, set the git credential helper at the start of the file to look for the mounted credential secret:
```docker
RUN git config --global credential.helper 'store --file=/run/secrets/git-credentials'
```

Then, for any commands which need git credentials to succeed, prefix the command with a secret mount like so:
```docker
RUN --mount=type=secret,id=git-credentials
```

An example usage could be when you need to clone a private git repo or download go modules from a private git repo:
```docker
RUN --mount=type=secret,id=git-credentials git clone https://github.com/username/repo.git
```

Paste the bash code below into your `~/.bash_profile`; it includes a `docker-build` function which accepts the same arguments as `docker build`, and two general helper functions `describe` and `try` :)

Once you've re-run `source ~/.bash_profile`, you can try it out with `docker-build -t repo/image:tag .`

```sh
export DOCKER_BUILDKIT=1

# docker-build is a wrapper which will safely inject git credentials at build time using git-credential-1password
docker-build() {
  local git_credentials_id="git-credentials"

  local usage="docker-build [...docker build args]"
  local descr="Wraps docker build with automatic git credential mounting. Add \`RUN git config --global credential.helper 'store --file=/run/secrets/$git_credentials_id'\` to the top of your Dockerfile and prefix any commands in your Dockerfile that need access to git credentials with \`RUN --mount=type=secret,id=$git_credentials_id\`."
  local exmpl="docker-build -t repo/image:tag ."
	describefn() { describe --usage "$usage" --descr "$descr" --exmpl "$exmpl"; }

	if [[ "$@" == "" ]]; then describefn; return 1; fi

	local git_credentials_src="git-credentials"

	printf $'protocol=https\nhost=github.com\n' | git-credential-1password get > $git_credentials_src

	key() {
		echo $(cat $git_credentials_src | grep "$1=" | sed "s/$1=//")
	}

	local protocol=$(key protocol)
	local username=$(key username)
	local password=$(key password)
	local host=$(key host)
	local path=$(key path)

	if [[ "$path" != "" ]]; then local path="/$path"; fi

	echo "$protocol://$username:$password@$host$path" > $git_credentials_src

	local cmd="docker build --secret id=$git_credentials_id,src=$git_credentials_src $@"
	try "$cmd" "rm $git_credentials_src"
}

# describe prints a formatted usage/example/description for a command
describe() {
	local cmd_usage="describe [--usage|-u USAGE] [--descr|-d DESCR] [--exmpl|-e EXMPL]"
	local cmd_descr="Format a cli's details by providing usage, description and/or example messages."
	local cmd_exmpl="describe --usage \"foo <whaaa>\" --descr \"Complains if whaaa doesn't equal \"bar\". --exmpl \"foo baz\""

	local usage=""
	local descr=""
	local exmpl=""

	while [[ $1 != "" ]]; do
		case $1 in
			--usage|-u) shift; local usage="$1"; shift;;
			--descr|-d) shift; local descr="$1"; shift;;
			--exmpl|-e) shift; local exmpl="$1"; shift;;
			*)          echo "Unknown flag $1." >&2; describe; return 1;;
		esac
	done

	local cols=$(tput cols)
	if [[ $usage == "" && $descr == "" && $exmpl == "" ]]; then
		describe --usage "$cmd_usage" --descr "$cmd_descr" --exmpl "$cmd_exmpl"
		return 0
	fi

	echo ""
	if [[ $usage != "" ]]; then
		fmt_usage=$(echo "         $usage" | fmt -w `expr $cols - 2`)
		echo "  Usage: ${fmt_usage:2}"
	fi
	if [[ $exmpl != "" ]]; then
		fmt_exmpl=$(echo "         $exmpl" | fmt -w `expr $cols - 2`)
		echo "  Exmpl: ${fmt_exmpl:2}"
	fi
	if [[ $descr != "" ]]; then
		fmt_descr=$(echo "         $descr" | fmt -w `expr $cols - 2`)
		echo "  Descr: ${fmt_descr:2}"
	fi
	echo ""
}

# try executes the first positional arg as a command and guarantees that the second
# positional arg will be called after the program stops, whether on return or interruption.
try() {
	local usage="try <command> <finally> [--verbose|-v]"
	local descr="Tries to run command, and guarantees running finally after return/termination/interruption."
	local exmpl="try 'cd ~ && echo \$PWD && sleep 3 && return 1' 'cd ~/Desktop && echo \$PWD'"
	describefn() { describe --usage "$usage" --descr "$descr" --exmpl "$exmpl"; }

	local command="$1"; shift
	local finally="$1"; shift
	if [[ $command == "" || "$finally" == "" ]]; then describefn; return 1; fi

	local verbose=0
	while [[ $1 != "" ]]; do
		case $1 in
			--verbose|-v) shift; local verbose=1;;
			*)            echo "Unknown flag $1." >&2; return 1;;
		esac
	done

	# Capture interruption signals produced while running
	# cmd so that we can run cleanup before ending
	trap "trap - RETURN; cleanup" RETURN

	# Cleanup - cd to original directory and clear trap
	cleanup() {
		eval $finally
		trap - SIGINT
		shopt -u extdebug
	}

	# Echo command if verbose is set
	if [[ $verbose == 1 ]]; then echo $command; fi

	# Capture command's return code
	shopt -s extdebug
	eval $command; rc=$?

	# Return command's return code
	return `expr $rc + 0`
}
```
