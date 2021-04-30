package test

import (
  "fmt"
  "strings"
  "testing"

  "github.com/stretchr/testify/require"
  "github.com/tlowerison/credential-1password/op"
)

func testOpFuncWithTest(fn func(stdin string, args []string)) op.OpFunc {
  return func(stdin string, args []string) (string, error) {
    fn(stdin, args)
    return "", nil
  }
}

func testOpFuncWithOutput(output string) op.OpFunc {
  return func(stdin string, args []string) (string, error) {
    return output, nil
  }
}

func testOpFuncWithErr(errMsg string) op.OpFunc {
  return func(stdin string, args []string) (string, error) {
    return "", fmt.Errorf(errMsg)
  }
}

func testOpFunc(stdin string, args []string) (string, error) {
  return "", nil
}

func TestShouldClearSessionAndRetry(t *testing.T) {
  require.Equal(t, false, op.ShouldClearSessionAndRetry(nil))
  require.Equal(t, false, op.ShouldClearSessionAndRetry(fmt.Errorf("")))
  require.Equal(t, false, op.ShouldClearSessionAndRetry(fmt.Errorf("[ERROR] 2021/04/29 14:42:46 foobar")))
  require.Equal(t, true, op.ShouldClearSessionAndRetry(fmt.Errorf("[ERROR] 2021/04/29 14:42:46 You are not currently signed in. Please run `op signin --help` for instructions")))
  require.Equal(t, true, op.ShouldClearSessionAndRetry(fmt.Errorf("[ERROR] 2021/04/29 14:42:46 Invalid session token")))
}

func TestCreateDocument(t *testing.T) {
  sessionToken := "session-token"
  vaultUUID := "vault-uuid"
  documentTitle := "document-title"
  documentFileName := "document-file-name"
  content := strings.Join([]string{"foobar", "abc=123"}, "\n")

  output, err := op.CreateDocument(
    testOpFuncWithTest(func(stdin string, args []string) {
      require.Equal(t, content, stdin)
      require.Equal(t, []string{"create", "document", "-", "--session", sessionToken, "--vault", vaultUUID, "--title", documentTitle, "--file-name", documentFileName}, args)
    }),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.CreateDocument(
    testOpFuncWithOutput(expOutput),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.Nil(t, err)
  require.Equal(t, expOutput, output)


  expErrMsg := "test-error-message"
  output, err = op.CreateDocument(
    testOpFuncWithErr(expErrMsg),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          VaultUUID: vaultUUID,
        },
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create document: missing session token", err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
        },
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create document: missing vault uuid", err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create document: missing document title", err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      Title: documentTitle,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create document: missing document file name", err.Error())
  require.Equal(t, "", output)
}

func TestDeleteDocument(t *testing.T) {
  sessionToken := "session-token"
  vaultUUID := "vault-uuid"
  documentTitle := "document-title"

  err := op.DeleteDocument(
    testOpFuncWithTest(func(stdin string, args []string) {
     require.Equal(t, "", stdin)
     require.Equal(t, []string{"delete", "document", documentTitle, "--session", sessionToken, "--vault", vaultUUID}, args)
    }),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID:    vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.Nil(t, err)

  expErrMsg := "test-error-message"
  err = op.DeleteDocument(
    testOpFuncWithErr(expErrMsg),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID:    vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())

  err = op.DeleteDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        VaultUUID: vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to delete document: missing session token", err.Error())

  err = op.DeleteDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to delete document: missing vault uuid", err.Error())

  err = op.DeleteDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID:    vaultUUID,
      },
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to delete document: missing document title", err.Error())
}

func TestEditDocument(t *testing.T) {
  sessionToken := "session-token"
  vaultUUID := "vault-uuid"
  documentTitle := "document-title"
  documentFileName := "document-file-name"
  content := strings.Join([]string{"foobar", "abc=123"}, "\n")

  output, err := op.EditDocument(
    testOpFuncWithTest(func(stdin string, args []string) {
      require.Equal(t, content, stdin)
      require.Equal(t, []string{"edit", "document", documentTitle, "-", "--session", sessionToken, "--vault", vaultUUID}, args)
    }),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
        Key: documentTitle,
      },
      Content: content,
    },
  )
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.EditDocument(
    testOpFuncWithOutput(expOutput),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
        Key: documentTitle,
      },
      Content: content,
    },
  )
  require.Nil(t, err)
  require.Equal(t, expOutput, output)

  expErrMsg := "test-error-message"
  output, err = op.EditDocument(
    testOpFuncWithErr(expErrMsg),
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
        Key: documentTitle,
      },
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)

  output, err = op.EditDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          VaultUUID: vaultUUID,
        },
        Key: documentTitle,
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to edit document: missing session token", err.Error())
  require.Equal(t, "", output)

  output, err = op.EditDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
        },
        Key: documentTitle,
      },
      Title: documentTitle,
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to edit document: missing vault uuid", err.Error())
  require.Equal(t, "", output)

  output, err = op.EditDocument(
    testOpFunc,
    op.DocumentUpsert{
      Query: op.Query{
        Context: op.Context{
          SessionToken: sessionToken,
          VaultUUID:    vaultUUID,
        },
      },
      FileName: documentFileName,
      Content: content,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to edit document: missing document title", err.Error())
  require.Equal(t, "", output)
}

func TestCreateVault(t *testing.T) {
  sessionToken := "session-token"
  title := "vault-title"
  description := "a vault description"

  output, err := op.CreateVault(
    testOpFuncWithTest(func(stdin string, args []string) {
      require.Equal(t, []string{"create", "vault", title, "--session", sessionToken, "--description", description, "--allow-admins-to-manage", "false"}, args)
    }),
    op.CreateVaultMutation{
      Description: description,
      SessionToken: sessionToken,
      Title: title,
    },
  )
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.CreateVault(
    testOpFuncWithOutput(expOutput),
    op.CreateVaultMutation{
      Description: description,
      SessionToken: sessionToken,
      Title: title,
    },
  )
  require.Nil(t, err)
  require.Equal(t, expOutput, output)

  expErrMsg := "test-error-message"
  output, err = op.CreateVault(
    testOpFuncWithErr(expErrMsg),
    op.CreateVaultMutation{
      Description: description,
      SessionToken: sessionToken,
      Title: title,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateVault(
    testOpFunc,
    op.CreateVaultMutation{
      Description: description,
      Title: title,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create vault: missing session token", err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateVault(
    testOpFunc,
    op.CreateVaultMutation{
      Description: description,
      SessionToken: sessionToken,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create vault: missing title", err.Error())
  require.Equal(t, "", output)

  output, err = op.CreateVault(
    testOpFunc,
    op.CreateVaultMutation{
      SessionToken: sessionToken,
      Title: title,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to create vault: missing description", err.Error())
  require.Equal(t, "", output)
}

func TestGetDocument(t *testing.T) {
  sessionToken := "session-token"
  vaultUUID := "vault-uuid"
  documentTitle := "document-title"

  output, err := op.GetDocument(
    testOpFuncWithTest(func(stdin string, args []string) {
      require.Equal(t, "", stdin)
      require.Equal(t, []string{"get", "document", documentTitle, "--session", sessionToken, "--vault", vaultUUID}, args)
    }),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID: vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.GetDocument(
    testOpFuncWithOutput(expOutput),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID: vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.Nil(t, err)
  require.Equal(t, expOutput, output)

  expErrMsg := "test-error-message"
  output, err = op.GetDocument(
    testOpFuncWithErr(expErrMsg),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID: vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)

  output, err = op.GetDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        VaultUUID: vaultUUID,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to get document: missing session token", err.Error())
  require.Equal(t, "", output)

  output, err = op.GetDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
      Key: documentTitle,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to get document: missing vault uuid", err.Error())
  require.Equal(t, "", output)

  output, err = op.GetDocument(
    testOpFunc,
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
        VaultUUID: vaultUUID,
      },
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to get document: missing document title", err.Error())
  require.Equal(t, "", output)
}

func TestGetVault(t *testing.T) {
  sessionToken := "session-token"
  vaultName := "vault-name"

  output, err := op.GetVault(
    testOpFuncWithTest(func(stdin string, args []string) {
      require.Equal(t, "", stdin)
      require.Equal(t, []string{"get", "vault", vaultName, "--session", sessionToken}, args)
    }),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
      Key: vaultName,
    },
  )
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.GetVault(
    testOpFuncWithOutput(expOutput),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
      Key: vaultName,
    },
  )
  require.Nil(t, err)
  require.Equal(t, expOutput, output)

  expErrMsg := "test-error-message"
  output, err = op.GetVault(
    testOpFuncWithErr(expErrMsg),
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
      Key: vaultName,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)

  output, err = op.GetVault(
    testOpFunc,
    op.Query{
      Context: op.Context{},
      Key: vaultName,
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to get vault: missing session token", err.Error())
  require.Equal(t, "", output)

  output, err = op.GetVault(
    testOpFunc,
    op.Query{
      Context: op.Context{
        SessionToken: sessionToken,
      },
    },
  )
  require.NotNil(t, err)
  require.Equal(t, "failed to get vault: missing vault name", err.Error())
  require.Equal(t, "", output)
}

func TestSignIn(t *testing.T) {
  output, err := op.Signin(testOpFuncWithTest(func(stdin string, args []string) {
    require.Equal(t, "", stdin)
    require.Equal(t, []string{"signin", "--raw"}, args)
  }))
  require.Nil(t, err)
  require.Equal(t, "", output)

  expOutput := "test-output"
  output, err = op.Signin(testOpFuncWithOutput(expOutput))
  require.Nil(t, err)
  require.Equal(t, expOutput, output)


  expErrMsg := "test-error-message"
  output, err = op.Signin(testOpFuncWithErr(expErrMsg))
  require.NotNil(t, err)
  require.Equal(t, expErrMsg, err.Error())
  require.Equal(t, "", output)
}
