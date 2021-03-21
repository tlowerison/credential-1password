# credential-1password

Credential helper for 1Password. Supports specifying which vault should store credentials.

### Install
`credential-1password` relies on 1Password's `op` CLI under the hood to manage credentials, first follow the steps to [set up + sign in with op](https://support.1password.com/command-line-getting-started).

#### Install for git
```sh
# pull binary and store in /usr/local/bin
wget https://github.com/tlowerison/credential-1password/releases/download/v1.0.0/git-credential-1password -q -O /usr/local/bin/git-credential-1password

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

#### Install for Docker
Update your docker version to at least `20.10.4`, there was a bug fix included that fixed docker from segfaulting when using custom credential helpers ([relevant pr](https://github.com/docker/cli/pull/2959)).
```sh
# logout of docker to remove old credentials
docker logout

# pull binary and store in /usr/local/bin
wget https://github.com/tlowerison/credential-1password/releases/download/v1.0.0/docker-credential-1password -q -O /usr/local/bin/docker-credential-1password

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
# > Enter the password for <my-1password@email.com> at my.1password.com: [type master password here]
# > Password: [type Personal Access Token here]

# confirm that your credentials are stored and retrievable
printf 'https://index.docker.io/v1/' | docker-credential-1password get
# > {"ServerURL":"https://index.docker.io/v1/","Username":"my-username","Secret":"my-secret"}

# finally, pull a private image from your logged-in registry
docker pull repo/image:tag
```
