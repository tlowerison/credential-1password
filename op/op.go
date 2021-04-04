package op

import (
  "fmt"
  "io"
  "os"
  "os/exec"
  "regexp"
  "strings"
)

type Context struct {
  SessionToken string
  VaultUUID    string
}

type Query struct {
  Context
  Key string // can be a title, uuid, etc.
}

type DocumentUpsert struct {
  Query
  Content  string
  FileName string
  Title    string
}

type CreateVaultMutation struct {
  Description  string
  SessionToken string
  Title        string
}

var retryRegexp = regexp.MustCompile("\\[ERROR\\] \\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2} You are not currently signed in. Please run `op signin --help` for instructions")

// op wraps 1Password's cli tool op.
func op(stdin string, args []string) (string, error) {
  cmd := exec.Command("op", args...)

  if stdin == "" {
    cmd.Stdin = os.Stdin
  } else {
    stdinPipe, err := cmd.StdinPipe()
    if err != nil {
      return "", err
    }

    go func() {
      defer stdinPipe.Close()
      io.WriteString(stdinPipe, stdin)
    }()
  }

  outBytes, err := cmd.CombinedOutput()
  output := string(outBytes)

  // err always has message "exit status 1"
  // actual error message captured in stdout
  if err != nil {
    if strings.HasPrefix(output, "[ERROR]") {
      return "", fmt.Errorf(output)
    }
    return "", err
  }

  return strings.TrimSpace(string(outBytes)), nil
}

// GetDocument wraps "op get document" and captures stdout/stderr
func GetVault(input Query) (string, error) {
  return op("", []string{
    "get", "vault", input.Key,
    "--session", input.SessionToken,
  })
}

// CreateVault creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func CreateVault(input CreateVaultMutation) (string, error) {
  return op("", []string{
    "create", "vault", input.Title,
    "--session", input.SessionToken,
    "--allow-admins-to-manage", "false",
    "--description", input.Description,
  })
}

// GetItem wraps "op get item" and captures stdout/stderr
func GetItem(input Query) (string, error) {
  return op("", []string{
    "get", "item", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
}

// GetDocument wraps "op get document" and captures stdout/stderr
func GetDocument(input Query) (string, error) {
  return op("", []string{
    "get", "document", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
}

// CreateDocument creates a new 1Passord document
// and returns the created login's uuid on success.
func CreateDocument(input DocumentUpsert) (string, error) {
  return op(input.Content, []string{
    "create", "document", "-",
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
    "--title", input.Title,
    "--file-name", input.FileName,
  })
}

// EditDocument edits a 1Passord document by uuid, name, etc.
// and returns the edited login's uuid on success.
func EditDocument(input DocumentUpsert) (string, error) {
  args := []string{
    "edit", "document", input.Key, "-",
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  }

  if input.FileName != "" {
    args = append(args, "--file-name", input.FileName)
  }

  if input.Title != "" {
    args = append(args, "--title", input.Title)
  }

  return op(input.Content, args)
}

// DeleteDocument deletes any document by uuid, name, etc.
func DeleteDocument(input Query) error {
  _, err := op("", []string{
    "delete", "document", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
  return err
}

// Signin requests the user to sign into 1Password through
// stdin, then returns the provided session token.
func Signin() (string, error) {
  return op("", []string{"signin", "--raw"})
}

// ShouldClearSessionAndRetry
func ShouldClearSessionAndRetry(err error) bool {
  return err != nil && retryRegexp.MatchString(err.Error())
}
