package main

import (
  "fmt"
  "net/url"

  "github.com/tidwall/gjson"
  "github.com/tlowerison/credential-1password/op"
  "github.com/tlowerison/credential-1password/util"
)

func Vault(ctx *util.Context, args []string) error {
  vaultName := ctx.GetVaultName()
  if len(args) == 0 && !ctx.ShouldCreateVault {
    fmt.Println(vaultName)
    return nil
  } else if len(args) == 0 && ctx.ShouldCreateVault {
    return ctx.SetVaultName(vaultName, true)
  }
  return ctx.SetVaultName(args[0], ctx.ShouldCreateVault)
}

// Get retrieves a credential from 1Password
func Get(ctx *util.Context) error {
  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return err
  }

  vaultUUID, err := ctx.GetVaultUUID()
  if err != nil {
    return err
  }

  outBytes, err := op.Get(sessionToken, false, "item", key, "--vault", vaultUUID)
  if err != nil {
    return err
  }

  output := string(outBytes)
  queryField := func(field string) string {
    query := fmt.Sprintf("details.fields.#(designation==\"%s\").value", field)
    return gjson.Get(output, query).String()
  }

  username := queryField("username")
  password := queryField("password")
  switch ctx.GetMode() {
  case util.GitMode:
    printGitCredentials(ctx, username, password)
    break
  case util.DockerMode:
    printDockerCredentials(ctx, username, password)
    break
  case util.NPMMode:
    printNPMCredentials(ctx, username, password)
  default:
    break
  }
  return nil
}

// Store upserts a credential in 1Password
func Store(ctx *util.Context) error {
  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return err
  }

  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  vaultUUID, err := ctx.GetVaultUUID()
  if err != nil {
    return err
  }

  username, err := ctx.GetUsername()
  if err != nil {
    return err
  }

  password, err := ctx.GetPassword()
  if err != nil {
    return err
  }

  item, err := op.Get(sessionToken, false, "item", key, "--vault", vaultUUID)
  if err != nil {
    return err
  }

  uuid := gjson.Get(item, "uuid").String()
  storedUsername := gjson.Get(item, "username").String()
  storedPassword := gjson.Get(item, "password").String()

  if username == storedUsername && password == storedPassword {
    return nil
  }

  opArgs := []string{fmt.Sprintf("title=%s", key), fmt.Sprintf("username=%s", username), fmt.Sprintf("password=%s", password), "--vault", vaultUUID}

  if uuid == "" {
    _, err = op.CreateLogin(sessionToken, opArgs...)
  } else {
    _, err = op.EditItem(sessionToken, uuid, opArgs...)
  }
  return err
}

// Erase removes a credential in 1Password
func Erase(ctx *util.Context) error {
  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return err
  }

  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  vaultUUID, err := ctx.GetVaultUUID()
  if err != nil {
    return err
  }

  item, err := op.Get(sessionToken, false, "item", key, "--vault", vaultUUID)
  if err != nil {
    return err
  }

  uuid := gjson.Get(item, "uuid").String()

  if item == "" {
    return fmt.Errorf("unable to find item")
  }

  _, err = op.DeleteItem(sessionToken, uuid, "--vault", vaultUUID)
  return err
}

// --- mode specific fns ---

func printGitCredentials(ctx *util.Context, username string, password string) error {
  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  URL, err := url.Parse(key)
  if err != nil {
    return err
  }
  if URL == nil {
    return fmt.Errorf("cannot parse url derived from credentials")
  }

  if URL.Scheme == "" || URL.Host == "" || username == "" || password == "" {
    return nil
  }

  if URL.Scheme != "" {
    fmt.Printf("protocol=%s\n", URL.Scheme)
  }
  if URL.Host != "" {
    fmt.Printf("host=%s\n", URL.Host)
  }
  if URL.Path != "" {
    fmt.Printf("path=%s\n", URL.Path)
  }
  if username != "" {
    fmt.Printf("username=%s\n", username)
  }
  if password != "" {
    fmt.Printf("password=%s\n", password)
  }
  return nil
}

func printDockerCredentials(ctx *util.Context, username string, password string) error {
  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  if key == "" || username == "" || password == "" {
    return nil
  }

  fmt.Printf("{\"ServerURL\":\"%s\",\"Username\":\"%s\",\"Secret\":\"%s\"}\n", key, username, password)
  return nil
}

func printNPMCredentials(ctx *util.Context, username string, password string) error {
  key, err := ctx.GetKey()
  if err != nil {
    return err
  }

  fmt.Printf("registry=%s\n", key)
  fmt.Printf("always-auth=true\n")

  if username != "" {
    fmt.Printf("email=%s\n", username)
  }
  if password != "" {
    fmt.Printf("_auth=%s\n", password)
  }
  return nil
}
