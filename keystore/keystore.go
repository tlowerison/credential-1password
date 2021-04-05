package keystore

import (
  "fmt"
  "runtime"
)

const serviceName = "credential-1password"

func Get(key string) (string, error) {
  switch runtime.GOOS {
  case "darwin":
    return getOnDarwin(key)
  case "linux":
    return getOnLinux(key)
  default:
    return "", fmt.Errorf("only darwin and linux platforms are supported currently")
  }
}

func Set(key string, value string) error {
  switch runtime.GOOS {
  case "darwin":
    return setOnDarwin(key, value)
  case "linux":
    return setOnLinux(key, value)
  default:
    return fmt.Errorf("only darwin and linux platforms are supported currently")
  }
}
