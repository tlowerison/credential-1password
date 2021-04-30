package keystore

import (
  "fmt"

  "github.com/keybase/go-keychain"
  "github.com/tidwall/gjson"
  "github.com/tidwall/sjson"
)

// getKeychainItem
func getKeychainItem(serviceName string) keychain.Item {
  item := keychain.NewItem()
  item.SetSecClass(keychain.SecClassGenericPassword)
  item.SetService(serviceName)
  item.SetAccount(serviceName)
  item.SetMatchLimit(keychain.MatchLimitOne)
  item.SetSynchronizable(keychain.SynchronizableNo)
  return item
}

// getOnDarwin
func getOnDarwin(serviceName string, key string) (string, error) {
  result, err := queryItem(serviceName)
  if err != nil || result == nil {
    return "", err
  }
  return gjson.Get(string(result.Data), key).String(), nil
}

func itemWithData(serviceName string, data string, key string, value string) (keychain.Item, error) {
  item := getKeychainItem(serviceName)
  data, err := sjson.Set(data, key, value)
  if err != nil {
    return keychain.Item{}, err
  }
  item.SetData([]byte(data))
  return item, nil
}

// queryItem
func queryItem(serviceName string) (*keychain.QueryResult, error) {
  item := getKeychainItem(serviceName)
  item.SetReturnData(true)
  results, err := keychain.QueryItem(item)
  if err != nil {
    return nil, err
  } else if len(results) == 0 {
    return nil, nil
  } else if len(results) != 1 {
    return nil, fmt.Errorf(ErrMultipleKeystoreItemsFound)
  }
  return &results[0], nil
}

// setOnDarwin
func setOnDarwin(serviceName string, key string, value string) error {
  result, err := queryItem(serviceName)

  // add new item
  if err == keychain.ErrorItemNotFound || result == nil {
    item, dataErr := itemWithData(serviceName, "{}", key, value)
    if dataErr != nil {
      return dataErr
    }
    return keychain.AddItem(item)
  } else if err != nil {
    return err
  }

  item, dataErr := itemWithData(serviceName, string(result.Data), key, value)
  if dataErr != nil {
    return dataErr
  }
  return keychain.UpdateItem(item, item)
}

func getOnLinux(serviceName string, key string) (string, error) { return "", fmt.Errorf(ErrWrongPlatform) }
func setOnLinux(serviceName string, key string, value string) error { return fmt.Errorf(ErrWrongPlatform) }
