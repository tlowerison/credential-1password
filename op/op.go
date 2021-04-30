package op

import (
  "fmt"
  "io"
  "os"
  "os/exec"
  "regexp"
  "strconv"
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
  AllowAdminsToManage bool
  Description         string
  SessionToken        string
  Title               string
}

var retryRegexps = []*regexp.Regexp{
  regexp.MustCompile("\\[ERROR\\] \\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2} You are not currently signed in. Please run `op signin --help` for instructions"),
  regexp.MustCompile("\\[ERROR\\] \\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2} Invalid session token"),
}

type OpFunc func(stdin string, args []string) (string, error)

// Op wraps 1Password's cli tool op.
func Op(stdin string, args []string) (string, error) {
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

// ShouldClearSessionAndRetry
func ShouldClearSessionAndRetry(err error) bool {
  if err == nil {
    return false
  }
  for _, retryRegexp := range retryRegexps {
    if retryRegexp.MatchString(err.Error()) {
      return true
    }
  }
  return false
}

// wrapped fns

// CreateDocument creates a new 1Passord document
// and returns the created login's uuid on success.
func CreateDocument(op OpFunc, input DocumentUpsert) (string, error) {
  baseErrMsg := "failed to create document"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.VaultUUID == ""    { return "", fmt.Errorf("%s: missing vault uuid", baseErrMsg) }
  if input.Title == ""        { return "", fmt.Errorf("%s: missing document title", baseErrMsg) }
  if input.FileName == ""     { return "", fmt.Errorf("%s: missing document file name", baseErrMsg) }

  return op(input.Content, []string{
    "create", "document", "-",
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
    "--title", input.Title,
    "--file-name", input.FileName,
  })
}

// CreateVault creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func CreateVault(op OpFunc, input CreateVaultMutation) (string, error) {
  baseErrMsg := "failed to create vault"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.Title == ""        { return "", fmt.Errorf("%s: missing title", baseErrMsg) }
  if input.Description == ""  { return "", fmt.Errorf("%s: missing description", baseErrMsg) }

  return op("", []string{
    "create", "vault", input.Title,
    "--session", input.SessionToken,
    "--description", input.Description,
    "--allow-admins-to-manage", strconv.FormatBool(input.AllowAdminsToManage),
  })
}

// DeleteDocument deletes any document by uuid, name, etc.
func DeleteDocument(op OpFunc, input Query) error {
  baseErrMsg := "failed to delete document"
  if input.SessionToken == "" { return fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.VaultUUID == ""    { return fmt.Errorf("%s: missing vault uuid", baseErrMsg) }
  if input.Key == ""          { return fmt.Errorf("%s: missing document title", baseErrMsg) }

  _, err := op("", []string{
    "delete", "document", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
  return err
}

// EditDocument edits a 1Passord document by uuid, name,
// etc. and returns the edited login's uuid on success.
func EditDocument(op OpFunc, input DocumentUpsert) (string, error) {
  baseErrMsg := "failed to edit document"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.VaultUUID == ""    { return "", fmt.Errorf("%s: missing vault uuid", baseErrMsg) }
  if input.Key == ""          { return "", fmt.Errorf("%s: missing document title", baseErrMsg) }

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

// GetItem wraps "op get item" and captures stdout/stderr
func GetItem(op OpFunc, input Query) (string, error) {
  baseErrMsg := "failed to get item"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.VaultUUID == ""    { return "", fmt.Errorf("%s: missing vault uuid", baseErrMsg) }
  if input.Key == ""          { return "", fmt.Errorf("%s: missing item title", baseErrMsg) }

  return op("", []string{
    "get", "item", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
}

// GetDocument wraps "op get document" and captures stdout/stderr
func GetDocument(op OpFunc, input Query) (string, error) {
  baseErrMsg := "failed to get document"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.VaultUUID == ""    { return "", fmt.Errorf("%s: missing vault uuid", baseErrMsg) }
  if input.Key == ""          { return "", fmt.Errorf("%s: missing document title", baseErrMsg) }

  return op("", []string{
    "get", "document", input.Key,
    "--session", input.SessionToken,
    "--vault", input.VaultUUID,
  })
}

// GetVault wraps "op get vault" and captures stdout/stderr
func GetVault(op OpFunc, input Query) (string, error) {
  baseErrMsg := "failed to get vault"
  if input.SessionToken == "" { return "", fmt.Errorf("%s: missing session token", baseErrMsg) }
  if input.Key == ""          { return "", fmt.Errorf("%s: missing vault name", baseErrMsg) }

  return op("", []string{
    "get", "vault", input.Key,
    "--session", input.SessionToken,
  })
}

// Signin requests the user to sign into 1Password through
// stdin, then returns the provided session token.
func Signin(op OpFunc) (string, error) {
  return op("", []string{"signin", "--raw"})
}
