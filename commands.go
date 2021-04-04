package main

import (
  "fmt"

  "github.com/tidwall/gjson"
  "github.com/tlowerison/credential-1password/op"
  "github.com/tlowerison/credential-1password/util"
)

// Get retrieves a credential from 1Password
func Get(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }

  document, err := op.GetDocument(*query)
  if err != nil {
    return err
  }

  fmt.Println(document)
  return nil
}

// Store upserts a credential in 1Password
func Store(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }

  // ignore error because missing documents are returned as errors
  item, _ := op.GetItem(*query)

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
    _, err = op.CreateDocument(input)
  } else {
    _, err = op.EditDocument(input)
  }

  return err
}

// Erase removes a credential in 1Password
func Erase(ctx *util.Context) error {
  query, err := ctx.GetOpQuery()
  if err != nil {
    return err
  }
  return op.DeleteDocument(*query)
}

// Vault gets/sets which 1Password vault credentials should be stored in.
func Vault(ctx *util.Context, args []string) error {
  vaultName := ctx.GetVaultName()
  if len(args) == 0 {
    if ctx.ShouldCreateVault {
      return ctx.SetVaultName(vaultName, true)
    } else {
      fmt.Println(vaultName)
      return nil
    }
  }
  return ctx.SetVaultName(args[0], ctx.ShouldCreateVault)
}
