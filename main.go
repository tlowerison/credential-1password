///usr/bin/env go run "$0" "$@"; exit "$?"
package main

import (
  "bufio"
  "fmt"
  "os"
  "os/exec"
  "net/url"
  "path"
  "regexp"
  "strings"
  "time"

  "github.com/spf13/cobra"
  "github.com/spf13/viper"
  "github.com/tidwall/gjson"
)

type Mode string

const (
  gitMode    Mode = "git"
  dockerMode Mode = "docker"
)

var ModeLdFlag = string(gitMode)
var mode = Mode(ModeLdFlag)

var app = fmt.Sprintf("%s-credential-1password", string(mode))

var homeDir string

// --- config ---

var configPath = ""
const timeFormat = time.UnixDate

var sessionToken string
const sessionTokenKey = "session-token"

var sessionTokenDateKey = fmt.Sprintf("%s.date", sessionTokenKey)
var sessionTokenValueKey = fmt.Sprintf("%s.value", sessionTokenKey)

const vaultKey = "vault"

var vaultName string
var vaultNameKey = fmt.Sprintf("%s.name", vaultKey)
var defaultVaultName = fmt.Sprintf("%s-credential", string(mode))

var vaultUUID string
var vaultUUIDKey = fmt.Sprintf("%s.uuid", vaultKey)

var vaultDescription = fmt.Sprintf("Contains credentials managed by %s.", app)
var missinVaultErrMsg = "no vault found with name: \"%s\"\ncreate a new vault with: %s vault <vault-name>"

// --- stdin inputs ---

var (
  key      string
  username string
  password string
  URL      *url.URL
)

func main() {
  RegisterConfig()

  rootCmd := &cobra.Command{
    Use:   app,
    Short: "credential helper for 1Password",
    Run: func(cmd *cobra.Command, _ []string) {
      fmt.Println(cmd.UsageString())
    },
  }

  rootCmd.AddCommand(&cobra.Command{
    Use:   "signin",
    Short: "signin into op with stdin",
    Long:  "signin into op with stdin; other commands will also prompt your master password but after processing stdin for credential related input",
    Args:  cobra.ExactArgs(0),
    Run: func(_ *cobra.Command, _ []string) {
      handleErr(Signin())
    },
  })

  sessionCmd := &cobra.Command{
    Use:   "session",
    Short: "get/set a session token",
  }

  sessionCmd.AddCommand(&cobra.Command{
    Use: "get",
    Short: "get the current session token, creates one if none exists",
    Args:  cobra.ExactArgs(0),
    Run:   asRun(SessionGet),
  })

  sessionCmd.AddCommand(&cobra.Command{
    Use:   "set",
    Short: "set a session token provided through stdin",
    Args:  cobra.ExactArgs(0),
    Run:   asRun(SessionSet),
  })

  rootCmd.AddCommand(sessionCmd)

  rootCmd.AddCommand(&cobra.Command{
    Use:   "vault",
    Short: "get/set the vault that credential uses",
    Args:  cobra.RangeArgs(0, 1),
    Run:   asRunWithArgs(Vault),
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:    "get",
    Short:  "get credential by key",
    PreRun: PreRun,
    Run:   asRun(Get),
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:    "store",
    Short:  "store key=value pair",
    PreRun: PreRun,
    Run:    asRun(Store),
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:    "erase",
    Short:  "erase credential by key",
    PreRun: PreRun,
    Run:    asRun(Erase),
  })

  rootCmd.Execute()
}

func RegisterConfig() {
  var err error
  homeDir, err = os.UserHomeDir()
  if err != nil {
    panic(err)
  }

  configPath = path.Join(homeDir, fmt.Sprintf(".%s", app))
  os.MkdirAll(configPath, 0700)

  viper.AddConfigPath(configPath)
  viper.SetConfigName("config")
  viper.SetConfigType("yaml")

  viper.SafeWriteConfig()
  err = viper.ReadInConfig()
  if err != nil {
    panic(fmt.Errorf("unable to read config: %s", err))
  }
}

// --- command wrappers ---

func asRun(fn func() error) func(_ *cobra.Command, _ []string) {
  return func(cmd *cobra.Command, args []string) {
    withSessionRetry(cmd, args, fn)
  }
}

func asRunWithArgs(fn func(args []string) error) func(_ *cobra.Command, args []string) {
  return func(cmd *cobra.Command, args []string) {
    withSessionRetry(cmd, args, func() error { return fn(args) })
  }
}

func withSessionRetry(cmd *cobra.Command, args []string, fn func() error) {
  err := fn()
  if shouldClearSessionAndRetry(err) {
    ClearSessionToken()
    if cmd.PreRun != nil {
      cmd.PreRun(cmd, args)
    }
    err = fn()
  }
  handleErr(err)
}

func handleErr(err error) {
  if err != nil {
    fmt.Fprintln(os.Stderr, err.Error())
    os.Exit(1)
  }
}

var retryRegexp = regexp.MustCompile("\\[ERROR\\] \\d{4}/\\d{2}/\\d{2} \\d{2}:\\d{2}:\\d{2} You are not currently signed in. Please run `op signin --help` for instructions")

func shouldClearSessionAndRetry(err error) bool {
  return err != nil && retryRegexp.MatchString(err.Error())
}

// --- command pre-runs ---

func PreRun(cmd *cobra.Command, args []string) {
  withSessionRetry(cmd, args, func() error { return preRun(cmd) })
}

func preRun(cmd *cobra.Command) error {
  err := ParseURL(cmd)
  if err != nil {
    return err
  }

  err = PreRunSessionToken()
  if err != nil {
    return err
  }

  name := viper.Get(vaultNameKey)
  shouldUpsertVault := (name == nil || name == "")
  if shouldUpsertVault {
    viper.Set(vaultNameKey, defaultVaultName)
  }

  err = PreRunVaultUUID(shouldUpsertVault)
  if err != nil {
    return err
  }
  return nil
}

func PreRunSessionToken() error {
  token, err := GetSessionToken()
  if err != nil {
    return err
  }

  if token == "" {
    token, err = CreateSessionToken()
  }
  if err != nil {
    return err
  }

  sessionToken = token
  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()

  return nil
}

func PreRunVaultUUID(shouldUpsert bool) error {
  vaultName = GetVaultName()
  uuid, err := GetVaultUUID(shouldUpsert)
  if err != nil {
    return err
  }
  if (!shouldUpsert && uuid == "") {
    return fmt.Errorf("unable to get specified vault's uuid")
  }

  if shouldUpsert && uuid == "" {
    uuid, err = UpsertVaultUUID()
  }
  if err != nil {
    return err
  }
  if uuid == "" {
    return fmt.Errorf("unable to get specified vault's uuid")
  }

  vaultUUID = uuid
  viper.Set(vaultUUIDKey, vaultUUID)
  viper.WriteConfig()

  return nil
}

// --- commands ---

func Signin() error {
  token, err := CreateSessionToken()
  if err != nil {
    return err
  }

  sessionToken = token
  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()

  return nil
}

func SessionGet() error {
  err := PreRunSessionToken()
  if err != nil {
    return err
  }
  if sessionToken != "" {
    fmt.Println(sessionToken)
  }
  return nil
}

func SessionSet() error {
  reader := bufio.NewReader(os.Stdin)
  line, err := reader.ReadString('\n')
  if err != nil && err.Error() != "EOF" {
    return err
  }

  sessionToken = strings.TrimSpace(line)
  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()
  return nil
}

func Vault(args []string) error {
  if len(args) == 0 {
    fmt.Println(GetVaultName())
  } else {
    PreRunSessionToken()

    vaultName = args[0]
    viper.Set(vaultNameKey, vaultName)
    viper.Set(vaultUUIDKey, "")
    viper.WriteConfig()

    PreRunVaultUUID(true)
    viper.WriteConfig()
  }
  return nil
}

// Get retrieves a credential from 1Password
func Get() error {
  outBytes, err := OpGet(false, "item", key, "--vault", vaultUUID)
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
  switch mode {
  case gitMode:
    if URL != nil && URL.Scheme != "" {
      fmt.Printf("protocol=%s\n", URL.Scheme)
    }
    if URL != nil && URL.Host != "" {
      fmt.Printf("host=%s\n", URL.Host)
    }
    if URL != nil && URL.Path != "" {
      fmt.Printf("path=%s\n", URL.Path)
    }
    if username != "" {
      fmt.Printf("username=%s\n", username)
    }
    if password != "" {
      fmt.Printf("password=%s\n", password)
    }
    break
  case dockerMode:
    var serverURL string
    if URL != nil {
      serverURL = URL.String()
    }
    fmt.Println(fmt.Sprintf(`{"ServerURL":"%s","Username":"%s","Secret":"%s"}`, serverURL, username, password))
    break
  default:
    break
  }
  return nil
}

// Store upserts a credential in 1Password
func Store() error {
  // value := fmt.Sprintf("%s:%s", username, password)
  item, err := OpGet(false, "item", key, "--vault", vaultUUID)
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
    _, err = Op(append([]string{"create", "item", "Login"}, opArgs...)...)
  } else {
    _, err = Op(append([]string{"edit", "item", uuid}, opArgs...)...)
  }
  return err
}

// Erase removes a credential in 1Password
func Erase() error {
  item, err := OpGet(false, "item", key, "--vault", vaultUUID)
  if err != nil {
    return err
  }

  uuid := gjson.Get(item, "uuid").String()

  if item == "" {
    return fmt.Errorf("unable to find item")
  }

  _, err = Op("delete", "item", uuid, "--vault", vaultUUID)
  return err
}

// --- cached value helpers ---

func GetVaultName() string {
  vaultNameIntf := viper.Get(vaultNameKey)
  if vaultNameIntf != nil {
    return vaultNameIntf.(string)
  }
  return ""
}

// GetVaultUUID gets the configured vault's uuid.
func GetVaultUUID(silent bool) (string, error) {
  vaultUUIDIntf := viper.Get(vaultUUIDKey)
  if vaultUUIDIntf != nil {
    uuid := vaultUUIDIntf.(string)
    if uuid != "" {
      return uuid, nil
    }
  }

  outBytes, err := OpGet(silent, "vault", vaultName)
  if err != nil {
    return "", err
  }

  return gjson.Get(string(outBytes), "uuid").String(), nil
}

// UpsertVaultUUID calls creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func UpsertVaultUUID() (string, error) {
  outBytes, err := Op(
    "create",
    "vault", vaultName,
    "--allow-admins-to-manage", "false",
    "--description", fmt.Sprintf("'%s'", vaultDescription),
  )
  if err != nil {
    return "", err
  }

  uuid := gjson.Get(string(outBytes), "uuid").String()
  return uuid, nil
}

// GetSessionToken retrieves the locally stored session token, if still valid.
func GetSessionToken() (string, error) {
  start := time.Now()
  dateIntf := viper.Get(sessionTokenDateKey)
  if dateIntf != nil {
    date, err := time.Parse(timeFormat, dateIntf.(string))
    if err == nil && start.Sub(date).Minutes() < 30 {
      tokenIntf := viper.Get(sessionTokenValueKey)
      if tokenIntf != nil {
        token := tokenIntf.(string)
        return token, nil
      }
    }
  }
  return "", nil
}

// CreateSessionToken requests the user to sign into 1Password through
// stdin, then stores the provided session token in the local config.
func CreateSessionToken() (string, error) {
  outBytes, err := Op("signin", "--raw")
  if err != nil {
    return "", err
  }

  return strings.TrimSpace(string(outBytes)), nil
}

func ClearSessionToken() {
  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, "")
  viper.WriteConfig()
}

// --- op helpers ---

// Op wraps 1Password's cli tool op.
func Op(args ...string) ([]byte, error) {
  cmd := exec.Command("op", append(args, "--session", sessionToken)...)
  cmd.Stdin = os.Stdin
  cmd.Stderr = os.Stderr
  return cmd.Output()
}

// OpGet wraps op get and hides stderr
func OpGet(silent bool, args ...string) (string, error) {
  cmd := exec.Command("op", append(append([]string{"get"}, args...), "--session", sessionToken)...)
  cmd.Stdin = os.Stdin

  outBytes, _ := cmd.CombinedOutput()
  out := strings.TrimSpace(string(outBytes))

  if !silent && strings.Contains(out, "doesn't seem to be a vault in this account") {
    err := fmt.Errorf(missinVaultErrMsg, vaultName, app)
    fmt.Fprintln(os.Stderr, err.Error())
    viper.Set(vaultUUIDKey, "")
    viper.WriteConfig()
    return "", err
  }
  if strings.HasPrefix(out, "[ERROR]") {
    err := fmt.Errorf(out)
    if shouldClearSessionAndRetry(err) {
      return "", err
    } else {
      return "", nil
    }
  }
  return out, nil
}

// --- helpers ---

// ParseURL reads from stdin and sets key, username and password.
func ParseURL(cmd *cobra.Command) error {
  // if key is not empty, we've already parsed input so
  // we don't want to parse again, that will clear key, username, etc.
  if key != "" {
    return nil
  }

  URL = &url.URL{}
  var rawurl string

  scanner := bufio.NewScanner(os.Stdin)
  lines := []string{}
	for scanner.Scan() {
    line := scanner.Text()
    lines = append(lines, line)
    if line == "" {
      break
    }
    if mode == gitMode {
      if strings.HasPrefix(line, "protocol=") { URL.Scheme = strings.TrimPrefix(line, "protocol=") }
      if strings.HasPrefix(line, "username=") { username   = strings.TrimPrefix(line, "username=") }
      if strings.HasPrefix(line, "password=") { password   = strings.TrimPrefix(line, "password=") }
      if strings.HasPrefix(line, "host=")     { URL.Host   = strings.TrimPrefix(line, "host=") }
      if strings.HasPrefix(line, "path=")     { URL.Path   = strings.TrimPrefix(line, "path=") }
      if strings.HasPrefix(line, "url=")      { rawurl     = strings.TrimPrefix(line, "url=") }
    }
  }
  os.Stdin.Close()

  if mode == dockerMode {
    input := strings.Join(lines, "\n")
    if cmd.Use == "store" {
      rawurl = strings.TrimSpace(gjson.Get(input, "ServerURL").String())
      username = strings.TrimSpace(gjson.Get(input, "Username").String())
      password = strings.TrimSpace(gjson.Get(input, "Secret").String())
    } else {
      rawurl = input
    }
  }

  if rawurl != "" {
    var err error
    URL, err = url.Parse(rawurl)
    if err != nil {
      return err
    }
    if URL.User.Username() != "" {
      username = URL.User.Username()
    }
    URL.User = nil
  }

  key = URL.String()
  return nil
}
