// +build darwin

package keystore

import (
  "fmt"

  "github.com/keybase/go-keychain"
)

// getKeychainItem
func getKeychainItem(key string) keychain.Item {
  item := keychain.NewItem()
  item.SetSecClass(keychain.SecClassGenericPassword)
  item.SetService(serviceName)
  item.SetAccount(key)
  item.SetMatchLimit(keychain.MatchLimitOne)
  item.SetSynchronizable(keychain.SynchronizableNo)
  return item
}

// getOnDarwin
func getOnDarwin(key string) (string, error) {
  item := getKeychainItem(key)
  item.SetReturnData(true)
  results, err := keychain.QueryItem(item)
  if err != nil {
    return "", err
  } else if len(results) != 1 {
    return "", nil
  }
  return string(results[0].Data), nil
}

// setOnDarwin
func setOnDarwin(key string, value string) error {
  item := getKeychainItem(key)
  item.SetData([]byte(value))

  err := keychain.AddItem(item)
  if err == keychain.ErrorDuplicateItem {
    return keychain.UpdateItem(item, item)
  }
  return err
}

func getOnLinux(key string) (string, error) { return "", fmt.Errorf("wrong platform") }
func setOnLinux(key string, value string) error { return fmt.Errorf("wrong platform") }
