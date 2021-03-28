# credential-1password

A credential helper which stores secrets in 1Password and interfaces seamlessly with both git and docker. 1Password issues session tokens which remain valid until unused for 30min, so development flows naturally since your master password is only requested for git / docker operations after periods of inactivity.

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

# unset existing credential.helper
git config -f $(git config --show-origin --get credential.helper | sed 's/file://' | sed 's/\t.*//') --unset credential.helper

# set as global credential store
git config --global credential.helper 1password

# Optional: set the name of the vault you want to store credentials in. Default: git-credential
# git-credential-1password vault <vault-name>

# store your credentials using key=value pairs passed into stdin
# - stdin then opens, will close after receiving two newlines
# - after stdin closes, you'll be asked to sign into 1Password if it's been 30 minutes since you last accessed 1Password with git-credential-1password
# Ex: git-credential-1password store
#   > protocol=https
#   > host=github.com
#   > username=my-username
#   > password=my-password # NOTE: you probably want to use a Github Personal Access Token here
#   >
#   > Enter the password for <my-1password@email.com> at my.1password.com: [type master password here]
git-credential-1password store

# confirm that your credentials are stored and retrievable
printf $'protocol=https\nhost=github.com\n' | git-credential-1password get
# > protocol=https
# > host=github.com
# > username=my-username
# > password=my-password

# clone a private repo
git clone https://github.com/username/repo.git
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
# NOTE: As of now, it's essential to use the '--username' flag instead of providing username through stdin.
# - Bug report out at https://github.com/docker/cli/issues/3022
# - Read more about the docker login command at https://docs.docker.com/engine/reference/commandline/login
#
# Ex: docker login --username=<my-username>
#   > Password: [type Personal Access Token here]
#   > Enter the password for <my-1password@email.com> at my.1password.com: [type master password here]
docker login --username=<my-username>

# confirm that your credentials are stored and retrievable
printf 'https://index.docker.io/v1/' | docker-credential-1password get
# > {"ServerURL":"https://index.docker.io/v1/","Username":"my-username","Secret":"my-secret"}

# pull an image from your private registry
docker pull repo/image:tag
```

## Use git credentials in docker builds

Combining `git-credential-1password` and [Docker BuildKit secrets](https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information) allows us to safely inject git credentials into containers at build time.

The idea is to wrap `docker build` with an alias `docker-build` which will:
- pull your git credentials with `git-credential-1password get`
- format those credentials into url format for use with the basic `store` git credential helper in our Dockerfile
- call `docker build` with (a) all the same arguments provided to `docker-build` and (b) our git credentials

In order to use the pulled git credentials, set the git credential helper at the start of your Dockerfile to look for the mounted credentials:
```docker
RUN git config --global credential.helper 'store --file=/run/secrets/git-credentials'
```

Then, for any commands which need git credentials to succeed, prefix the command with a secret mount like so:
```docker
RUN --mount=type=secret,id=git-credentials git clone https://github.com/username/repo.git
```

Once your Dockerfile's all set, build with `docker-build -t repo/image:tag .`

Paste the code below into your `~/.bash_profile`, then run `source ~/.bash_profile` to use `docker-build`.

```sh
docker-build() {
  # docker secret id
  local git_credentials_id="git-credentials"

  # docker secret file path
  local git_credentials_src="git-credentials"

  # get git-credentials and store them in a temporary file
  printf $'protocol=https\nhost=github.com\n' | git-credential-1password get > $git_credentials_src

  # retrieves a key from the temporary file
  gitkey() { echo $(cat $git_credentials_src | grep "$1=" | sed "s/$1=//"); }

  local protocol=$(gitkey protocol)
  local username=$(gitkey username)
  local password=$(gitkey password)
  local host=$(gitkey host)
  local path=$(gitkey path)

  # path does not include an initial / by default
  if [[ "$path" != "" ]]; then local path="/$path"; fi

  # reformat the credentials in the temporary file to use the url format expected by credential.helper store
  echo "$protocol://$username:$password@$host$path" > $git_credentials_src

  # try clause: build docker image
  local try="DOCKER_BUILDKIT=1 docker build --secret id=$git_credentials_id,src=$git_credentials_src $@"

  # finally clause: remove git-credentials file
  local finally="rm $git_credentials_src"

  # capture interruption and return signals produced
  # in the try clause so we can run cleanup at the end
  trap "trap - RETURN; cleanup" RETURN

  # executes finally clause and unsets extdebug
  cleanup() {
    shopt -u extdebug
    eval $finally
  }

  # set extdebug to trap return signals
  shopt -s extdebug

  # execute try clause
  eval $try
}
```
