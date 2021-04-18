// +build linux
// WIP

package keystore

import (
  "fmt"

  "github.com/keybase/go-keychain/secretservice"
)

// getAttributes
func getAttributes(key string) secretservice.Attributes {
  return map[string]string{"name": key}
}

// getOnLinux
func getOnLinux(key string) (string, error) {
  service, err := secretservice.NewService()
  if err != nil {
    return "", err
  }
  session, err := service.OpenSession(secretservice.AuthenticationDHAES)
  if err != nil {
    return "", err
  }
  defer service.CloseSession(session)

  items, err := service.SearchCollection(secretservice.SecretServiceObjectPath, getAttributes(key))
  if err != nil {
    return "", err
  } else if len(items) != 1 {
    return "", nil
  }

  item, err := service.GetSecret(items[0], *session)
  if err != nil {
    return "", err
  }

  return string(item), nil
}

// setOnLinux
func setOnLinux(key string, value string) error {
  service, err := secretservice.NewService()
  if err != nil {
    fmt.Println(err.Error())
    return err
  }
  session, err := service.OpenSession(secretservice.AuthenticationDHAES)
  if err != nil {
    fmt.Println(err.Error())
    return err
  }
  defer service.CloseSession(session)

  secret, err := session.NewSecret([]byte(value))
  if err != nil {
    fmt.Println(err.Error())
    return err
  }

  _, err = service.CreateItem(
    secretservice.SecretServiceObjectPath,
    secretservice.NewSecretProperties(key, getAttributes(key)),
    secret,
    secretservice.ReplaceBehaviorReplace,
  )
  return err
}

func getOnDarwin(key string) (string, error) { return "", fmt.Errorf("wrong platform") }
func setOnDarwin(key string, value string) error { return fmt.Errorf("wrong platform") }
