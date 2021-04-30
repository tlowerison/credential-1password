package keystore

import (
  "fmt"
  "runtime"
)

const serviceName = "credential-1password"
const ErrMultipleKeystoreItemsFound = "multiple keystore items found"
const ErrWrongPlatform = "wrong platform"

type Keystore interface {
  Get(key string) (string, error)
  Set(key string, value string) error
}

type keystore struct {
  serviceName string
}

func NewKeystore(serviceName string) keystore {
  return keystore{
    serviceName: serviceName,
  }
}

func (ks keystore) Get(key string) (string, error) {
  switch runtime.GOOS {
  case "darwin":
    return getOnDarwin(ks.serviceName, key)
  case "linux":
    return getOnLinux(ks.serviceName, key)
  default:
    return "", fmt.Errorf("only darwin and linux platforms are supported currently")
  }
}

func (ks keystore) Set(key string, value string) error {
  switch runtime.GOOS {
  case "darwin":
    return setOnDarwin(ks.serviceName, key, value)
  case "linux":
    return setOnLinux(ks.serviceName, key, value)
  default:
    return fmt.Errorf("only darwin and linux platforms are supported currently")
  }
}

type mockKeystore struct {
  Err   error
  Items map[string]string
}

func NewMockKeystore(err error, items map[string]string) *mockKeystore {
  if items != nil {
    return &mockKeystore{
      Err:   err,
      Items: items,
    }
  }
  return &mockKeystore{
    Err:   err,
    Items: map[string]string{},
  }
}

func (ks *mockKeystore) Get(key string) (string, error) {
  if ks.Err != nil {
    return "", ks.Err
  }
  if val, ok := ks.Items[key]; !ok {
    return "", fmt.Errorf(ErrWrongPlatform)
  } else {
    return val, nil
  }
}

func (ks *mockKeystore) Set(key string, value string) error {
  if ks.Err != nil {
    return ks.Err
  }
  ks.Items[key] = value
  return nil
}
