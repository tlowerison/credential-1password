// +build darwin

package keystore

import (
  "fmt"

  "github.com/keybase/go-keychain"
  "github.com/tidwall/gjson"
  "github.com/tidwall/sjson"
)

// getKeychainItem
func getKeychainItem() keychain.Item {
  item := keychain.NewItem()
  item.SetSecClass(keychain.SecClassGenericPassword)
  item.SetService(serviceName)
  item.SetAccount(serviceName)
  item.SetMatchLimit(keychain.MatchLimitOne)
  item.SetSynchronizable(keychain.SynchronizableNo)
  return item
}

// getOnDarwin
func getOnDarwin(key string) (string, error) {
  result, err := queryItem()
  if err != nil || result == nil {
    return "", err
  }
  return gjson.Get(string(result.Data), key).String(), nil
}

func itemWithData(data string, key string, value string) (keychain.Item, error) {
  item := getKeychainItem()
  data, err := sjson.Set(data, key, value)
  if err != nil {
    return keychain.Item{}, err
  }
  item.SetData([]byte(data))
  return item, nil
}

// queryItem
func queryItem() (*keychain.QueryResult, error) {
  item := getKeychainItem()
  item.SetReturnData(true)
  results, err := keychain.QueryItem(item)
  if err != nil {
    return nil, err
  } else if len(results) == 0 {
    return nil, nil
  } else if len(results) != 1 {
    return nil, fmt.Errorf("multiple keychain items found")
  }
  return &results[0], nil
}

// setOnDarwin
func setOnDarwin(key string, value string) error {
  result, err := queryItem()

  // add new item
  if err == keychain.ErrorItemNotFound || result == nil {
    item, dataErr := itemWithData("{}", key, value)
    if dataErr != nil {
      return dataErr
    }
    return keychain.AddItem(item)
  } else if err != nil {
    return err
  }

  item, dataErr := itemWithData(string(result.Data), key, value)
  if dataErr != nil {
    return dataErr
  }
  return keychain.UpdateItem(item, item)
}

func getOnLinux(key string) (string, error) { return "", fmt.Errorf("wrong platform") }
func setOnLinux(key string, value string) error { return fmt.Errorf("wrong platform") }
