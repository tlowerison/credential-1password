# git-credential-1password

git-credential helper for 1Password. Supports specifying which vault should store credentials.

### Install
```
wget https://github.com/tlowerison/git-credential-1password/releases/download/v1.0.0/git-credential-1password -q -O /usr/local/bin/git-credential-1password
chmod u+x /usr/local/bin/git-credential-1password
source ~/.bash_profile
```

### Usage
```
Usage:
  git-credential-1password [flags]
  git-credential-1password [command]

Available Commands:
  erase       erase credential by key
  get         retrieve credential by key
  help        Help about any command
  store       store key/credential pair
  vault       get/set the vault that git-credential uses

Flags:
  -h, --help   help for git-credential-1password

Use "git-credential-1password [command] --help" for more information about a command.
```
