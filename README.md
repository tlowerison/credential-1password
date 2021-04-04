# credential-1password

A credential helper which stores secrets in 1Password and interfaces seamlessly with both git and docker. 1Password issues session tokens which remain valid until unused for 30min, so development flows naturally since your master password is only requested for git / docker operations after periods of inactivity.

## Install
credential-1password relies on 1Password's `op` CLI under the hood to manage credentials, first follow the steps to [set up + sign in with op](https://support.1password.com/command-line-getting-started).

```sh
# pull binary and store in /usr/local/bin
wget https://github.com/tlowerison/credential-1password/releases/download/v1.0.4/credential-1password -q -O /usr/local/bin/credential-1password

# give executable permission
chmod u+x /usr/local/bin/credential-1password

# reload PATH
source ~/.bash_profile
```

### Install for predefined modes
Predefined modes include `git` and `docker`, they're only "predefined" because the two tools each expect their own particular stdin/stdout interface for these helpers. Any other mode will just use 1Password as a remote filestore.

You can copy/download and run the script in `install.sh` like so:
```sh
path/to/install.sh git docker
```

This will just create some files in `/usr/local/bin` called `git-credential-1password` and `docker-credential-1password` which just call `credential-1password $@ --mode=$mode` under the hood.

## Use credentials in docker builds

Combining `credential-1password` and [Docker BuildKit secrets](https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information) allows us to safely inject credentials into containers at build time.

The `docker-build` script includes a handy wrapper which looks for directory level then home level credential-1password configs for pulling secrets to pass into docker at build time. The config files should contain the info passed to `credential-1password get --mode=$mode` per mode you want to provide in the docker build. Ex:
```sh
# ~/.credential-1password/credentials
git
protocol=https
host=github.com

npm
```

Because `npm` is not a predefined mode, `credential-1password` will just store your `.npmrc` as a file and output that same file on `credential-1password get --mode=npm`, so we don't need to provide any additional keys for that config. Each mode defined in the config will be passed into your build with the id `$mode-credentials` (e.g. for git, `git-credentials`).

`docker-build` will first check for a directory level config at `$PWD/.credentials` and then for a home level config should at `~/.credential-1password/credentials`.
