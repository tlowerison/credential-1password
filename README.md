# credential-1password

A credential helper which stores secrets in 1Password and interfaces seamlessly with both git and docker. Also serves as a remote file store for any other types of credentials you wish to store (e.g. npm).

1Password issues session tokens which remain valid until unused for 30min, so your master password is only requested after periods of inactivity. Session tokens are automatically stored in your OS's encrypted keystore. Currently Keychain on darwin (Apple devices) is the only keystore supported, up next is maybe gnome-keyring for Linux. Interfacing with the keystore is mostly handled by https://pkg.go.dev/github.com/keybase/go-keychain.

## Install
credential-1password relies on 1Password's `op` tool under the hood to manage credentials, first follow the steps to [set up + sign in with op](https://support.1password.com/command-line-getting-started).

```sh
wget -q -O /usr/local/bin/credential-1password https://github.com/tlowerison/credential-1password/releases/download/v1.0.4/credential-1password-darwin
chmod u+x /usr/local/bin/credential-1password
```

### Install for predefined modes
Predefined modes include `git` and `docker`, they're only "predefined" because the two tools each expect their own particular stdin/stdout interface for these helpers. Any other mode will just use 1Password as a remote filestore.

You can run the script in `install.sh` with:
```sh
bash <(curl -s https://raw.githubusercontent.com/tlowerison/credential-1password/main/install.sh) git docker
```

Definitely go check out the code for `install.sh` to make sure you're comfortable running the command above. This will create a file per mode in `/usr/local/bin` called `$mode-credential-1password` respectively which calls `credential-1password $@ --mode=$mode` under the hood. It also configures `git` and `docker` properly to use `credential-1password` as their credential helper.

Note: `install.sh` will log you out of docker and ask you to log back in afterward. If you don't want to be logged out of docker by running this, download the script instead and modify it to your liking :)

### Use with other modes
Other modes beside `git` and `docker` will effectively use 1Password as a remote filestore. No input from stdin is required for calls to `credential-1password get` and `credential-1password erase` in this case, and the contents passed to `credential-1password store` will be saved as a document with whatever mode is provided. This is useful for systems which expect a local file as configuration, e.g. npm or yarn. For example:
```sh
$ printf $'@scope:registry=https://registry.yarnpkg.com/
_authToken=<auth-token-here>
always-auth=true' | credential-1password --mode=npm store

$ credential-1password --mode=npm get
> @scope:registry=https://registry.yarnpkg.com/
> _authToken=<auth-token-here>
> always-auth=true
```

## Use credentials in docker builds

Combining `credential-1password` and [Docker BuildKit secrets](https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information) allows us to safely inject credentials into containers at build time.

The `docker-build` script provided in this repo includes a handy wrapper which looks for credential-1password config files for pulling secrets to pass into docker at build time.

You can add it to your path with
```sh
wget -q -O /usr/local/bin/docker-build https://raw.githubusercontent.com/tlowerison/credential-1password/main/docker-build
chmod u+x /usr/local/bin/docker-build
```

The script looks for a file named `.credentials`, using the first one found amongst:
1. the current directory
2. (if upward tree search is enabled) all ancestor directories of the current directory up to `$HOME`
3. `$HOME/.credential-1password`

Upward tree search can be enabled with `credential-1password config docker-build.credentials-tree-search true`.

The `.credentials` config file should contain the info passed to `credential-1password get --mode=$mode` per mode you want to provide in the docker build. For example:
```
git
protocol=https
host=github.com

npm
```
will provide credentials found for `https://github.com` and `npm`. Each mode defined in the config will be passed into your build with the id `$mode-credentials` (e.g. for git, `git-credentials`) and can be located at `/run/secrets/$mode-credentials`.
