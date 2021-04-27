# credential-1password

A credential helper which stores secrets in 1Password and interfaces seamlessly with both git and docker. Also serves as a remote file store for any other types of credentials you wish to store (e.g. npm).

1Password issues session tokens which remain valid until unused for 30min, so your master password is only requested after periods of inactivity. Session tokens are automatically stored in your OS's encrypted keystore. Currently Keychain on darwin (Apple devices) is the only keystore supported, up next is maybe gnome-keyring for Linux. Interfacing with the keystore is mostly handled by https://pkg.go.dev/github.com/keybase/go-keychain.

## Install
credential-1password relies on 1Password's `op` tool under the hood to manage credentials, first follow the steps to [set up + sign in with op](https://support.1password.com/command-line-getting-started). Then download one of the release archive files:
- for MacOS, the .pkg file will automatically install `credential-1password`, `git-credential-1password`, `docker-credential-1password` and `docker-build` (see [below](https://github.com/tlowerison/credential-1password/#use-credentials-in-docker-builds))
- otherwise use the .zip file - unzip and move its contents into PATH

### Setup with git
```sh
# unset existing credential.helper
git config -f $(git config --show-origin --get credential.helper | sed 's/file://' | sed 's/\t.*//') --unset credential.helper
# set as global credential store
git config --global credential.helper 1password
```

### Setup with docker
1. Run `docker logout`.
2. In ~/.docker/config.json, set credsStore to `"1password"`.
3. Run `docker login --username=<your-username>`.
  - NOTE: using the --username flag here (as opposed to passing it in with stdin) is important at this time due to a bug in the docker cli

### Use with other modes
Other modes beside `git` and `docker` will effectively use 1Password as a remote filestore. No input from stdin is required for calls to `credential-1password get` and `credential-1password erase` in this case, and the contents passed to `credential-1password store` will be saved as a document with whatever mode is provided. This is useful for systems which expect a local file as configuration, e.g. npm or yarn. For example:
```sh
$ echo $'@scope:registry=https://registry.yarnpkg.com/
_authToken=<auth-token-here>
always-auth=true
' | credential-1password --mode=npm store

$ credential-1password --mode=npm get
> @scope:registry=https://registry.yarnpkg.com/
> _authToken=<auth-token-here>
> always-auth=true
```

## Use credentials in docker builds
Combining `credential-1password` and [Docker BuildKit secrets](https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information) allows us to safely inject credentials into containers at build time. `docker-build` is a script that comes included with the release which wraps `docker build` with credential-1password integration. It searches up the file tree for a file named `.credentials` which contains the keys used for `credential-1password get` (starting with the current directory and stopping once hitting `$HOME`; if the current directory is not a descendant of `$HOME`, only the current directory is checked). An example `.credentials` file for a nodejs project could look like this:
```
git
protocol=https
host=github.com

npm
```

This will provide the credentials found for `https://github.com` and `npm`. Each mode defined in the config (in this example they are `git` and `npm`) will be passed into your build with the id `$mode-credentials` (e.g. for git, `git-credentials`) and can be located at `/run/secrets/$mode-credentials`.

### Example
`.credentials`
```
git
protocol=https
host=github.com
```

`Dockerfile`
```dockerfile
FROM golang:1.16 AS build

WORKDIR /go/src/app

RUN git config --global credential.helper 'store --file=/run/secrets/git-credentials'

COPY go.mod go.sum ./
RUN --mount=type=secret,id=git-credentials go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/app ./...

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin /go/bin

ENV PATH="/go/bin:{PATH}"

CMD ["app"]
```

Run:
```sh
echo $'protocol=https
host=github.com
username=my-username
password=my-password' | git-credential-1password store

docker-build -t repo/image:tag .
```
