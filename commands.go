package main

import (
  "fmt"

  "github.com/tidwall/gjson"
  "github.com/tlowerison/credential-1password/op"
  "github.com/tlowerison/credential-1password/util"
)

var ConfigKeys = []string{
  util.VaultKey,
}

// Get retrieves a credential from 1Password
func Get(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }

  // err should only occur if document
  // does not exist, don't want to log
  document, err := op.GetDocument(ctx.OpFunc, *query)
  if err == nil {
    fmt.Println(document)
  }
  return nil
}

func Op(ctx *util.Context, args []string) error {
  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return err
  }

  output, err := op.Op("", append([]string{"--session", sessionToken}, args...))
  if err == nil {
    fmt.Println(output)
  }
  return err
}

// Store upserts a credential in 1Password
func Store(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }

  // ignore error because missing documents are returned as errors
  item, _ := op.GetItem(ctx.OpFunc, *query)

  uuid := gjson.Get(item, "uuid").String()
  mode := string(ctx.GetMode())

  title := query.Key
  query.Key = uuid
  input := op.DocumentUpsert{
    Query:    *query,
    Content:  ctx.GetInput(),
    FileName: fmt.Sprintf("%s-credentials", mode),
    Title:    title,
  }

  if uuid == "" {
    _, err = op.CreateDocument(ctx.OpFunc, input)
  } else {
    _, err = op.EditDocument(ctx.OpFunc, input)
  }

  return err
}

// Erase removes a credential in 1Password
func Erase(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }
  return op.DeleteDocument(ctx.OpFunc, *query)
}

func Config(ctx *util.Context, args []string) error {
  key := args[0]
  switch key {
  case util.VaultKey:
    return ConfigVault(ctx, args[1:])
  default:
    return fmt.Errorf("unknown config option %s", args[0])
  }
}

// ConfigVault gets/sets which 1Password vault credentials should be stored in.
func ConfigVault(ctx *util.Context, args []string) error {
  vaultName, err := ctx.GetVaultName()
  if err != nil {
    return err
  }

  if len(args) == 0 {
    if ctx.Flags.Config_Vault_Create {
      return ctx.SetVaultName(vaultName, true)
    } else {
      fmt.Println(vaultName)
      return nil
    }
  }

  return ctx.SetVaultName(args[0], ctx.Flags.Config_Vault_Create)
}
