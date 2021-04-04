#!/bin/bash

dependencies=("op credential-1password")
resources=("https://support.1password.com/command-line-getting-started" "https://github.com/tlowerison/credential-1password/releases")

missing_dependencies=0; i=0
for dependency in $dependencies; do
  if [[ "$(which "$dependency")" == "" ]]; then
    if [[ "$missing_dependencies" == "0" ]]; then
      echo "failed to install: missing dependencies:"
    fi
    missing_dependencies=1
    echo "- $dependency: ${resources[$i]}"
  fi
  ((i=i+1))
done

if [[ "$missing_dependencies" == "1" ]]; then
  exit 1
fi

for mode in $@; do
  case $mode in
    git);;
    docker);;
    *) echo "failed to install: unknown mode $mode"; exit 1 ;;
  esac
done

for mode in $@; do
  # create mode-credential-1password from existing credential-1password binary
  src="/usr/local/bin/$mode-credential-1password"
  echo "#!/bin/sh" > $src
  echo "credential-1password \$@ --mode=$mode" >> $src
  # make executable
  chmod 700 $src

  case $mode in
    git)
      # unset existing credential.helper
      git config -f $(git config --show-origin --get credential.helper | sed 's/file://' | sed 's/\t.*//') --unset credential.helper
      # set as global credential store
      git config --global credential.helper 1password
      ;;
    docker)
      if [[ "$(which jq)" == "" ]]; then
        echo "  "
        echo "  steps to finish installation for docker:"
        echo "  1. run \"docker logout\""
        echo "  2. change the value of credsStore in ~/.docker/config.json to \"1password\""
        echo "  3. run \"docker login --username=<your-username>\""
        echo "     - note that using the --username flag is important at this time due to a bug in the docker cli"
        echo "  "
        exit 0
      fi
      if [[ "$(cat ~/.docker/config.json | jq -r '.credsStore')" != "1password" ]]; then
        # logout of docker
        docker logout
        # update credsStore in docker config
        jq --argjson credsStore '"1password"' 'setpath(["credsStore"]; $credsStore)' ~/.docker/config.json > ~/.docker/.tmp.json && mv ~/.docker/.tmp.json ~/.docker/config.json
      fi
      echo "  "
      echo "  steps to finish installation for docker:"
      echo "  1. run \"docker login --username=<your-username>\""
      echo "     - note that using the --username flag is important at this time due to a bug in the docker cli"
      echo "  "
      ;;
    *)
      echo "failed to install: unknown mode $mode"
      exit 1
      ;;
  esac
done
