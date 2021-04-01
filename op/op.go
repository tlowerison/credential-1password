package op

import (
  "fmt"
  "os"
  "os/exec"
  "regexp"
  "strings"

  "github.com/tidwall/gjson"
)

var retryRegexp = regexp.MustCompile("\\[ERROR\\] \\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2} You are not currently signed in. Please run `op signin --help` for instructions")

// op wraps 1Password's cli tool op.
func op(sessionToken string, args ...string) (string, error) {
  newArgs := args
  if sessionToken != "" {
    newArgs = append(newArgs, "--session", sessionToken)
  }

  cmd := exec.Command("op", newArgs...)
  cmd.Stdin = os.Stdin
  cmd.Stderr = os.Stderr
  outputBytes, err := cmd.Output()
  return string(outputBytes), err
}

// Get wraps op get and hides stderr
func Get(sessionToken string, silent bool, args ...string) (string, error) {
  newArgs := args
  if sessionToken != "" {
    newArgs = append(newArgs, "--session", sessionToken)
  }

  cmd := exec.Command("op", append([]string{"get"}, newArgs...)...)
  cmd.Stdin = os.Stdin

  outBytes, _ := cmd.CombinedOutput()
  out := strings.TrimSpace(string(outBytes))

  if !silent && strings.Contains(out, "doesn't seem to be a vault in this account") {
    return "", fmt.Errorf(out)
  }
  if strings.HasPrefix(out, "[ERROR]") {
    err := fmt.Errorf(out)
    if ShouldClearSessionAndRetry(err) {
      return "", err
    } else {
      return "", nil
    }
  }
  return out, nil
}

// CreateLogin creates a new 1Passord login and returns
// the newly created login's uuid on success.
func CreateLogin(sessionToken string, args ...string) (string, error) {
  return op(sessionToken, append([]string{"create", "item", "Login"}, args...)...)
}

// CreateVault creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func CreateVault(sessionToken string, name string, description string) (string, error) {
  output, err := op(
    sessionToken,
    "create",
    "vault", name,
    "--allow-admins-to-manage", "false",
    "--description", description,
  )
  if err != nil {
    return "", err
  }

  vaultUUID := gjson.Get(output, "uuid").String()
  return vaultUUID, nil
}

// Signin requests the user to sign into 1Password through
// stdin, then returns the provided session token.
func Signin() (string, error) {
  output, err := op("", "signin", "--raw")
  if err != nil {
    return "", err
  }

  return strings.TrimSpace(output), nil
}

// EditItem edits an item, referenced by its uuid.
func EditItem(sessionToken string, uuid string, args ...string) (string, error) {
  return op(sessionToken, append([]string{"edit", "item", uuid}, args...)...)
}

// DeleteItem deletes any item by uuid, name, etc.
func DeleteItem(sessionToken string, args ...string) (string, error) {
  return op(sessionToken, append([]string{"delete", "item"}, args...)...)
}

// ShouldClearSessionAndRetry
func ShouldClearSessionAndRetry(err error) bool {
  return err != nil && retryRegexp.MatchString(err.Error())
}
