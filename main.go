package main

import (
  "bufio"
  "fmt"
  "os"
  "os/exec"
  "net/url"
  "path"
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

// util

var homeDir string
const whitespace = " \n\t"

// config

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
var missinVaultErrMsg = fmt.Sprintf("missing vault: \"%s\"\ncreate a new vault with: %s vault <vault-name>", app)

// inputs

var (
  key      string
  username string
  password string
  URL      *url.URL
)

// main

func main() {
  RegisterConfig()

  rootCmd := &cobra.Command{
    Use:   app,
    Short: fmt.Sprintf("credential helper for 1Password.", app),
    Run: func(cmd *cobra.Command, _ []string) {
      fmt.Println(cmd.UsageString())
    },
  }

  rootCmd.AddCommand(&cobra.Command{
    Use:   "vault",
    Short: "get/set the vault that credential uses",
    Args: cobra.RangeArgs(0, 1),
    Run: Vault,
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:   "get",
    Short: "get credential by key",
    PreRun: PreRun,
    Run: Get,
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:   "store",
    Short: "store key=value pair",
    PreRun: PreRun,
    Run: Store,
  })

  rootCmd.AddCommand(&cobra.Command{
    Use:   "erase",
    Short: "erase credential by key",
    PreRun: PreRun,
    Run: Erase,
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

// pre-run

func PreRun(cmd *cobra.Command, _ []string) {
  ParseURL(cmd)
  PreRunSessionToken()
  name := viper.Get(vaultNameKey)
  shouldUpsertVault := (name == nil || name == "")
  if shouldUpsertVault {
    viper.Set(vaultNameKey, defaultVaultName)
  }
  PreRunVaultUUID(shouldUpsertVault)
}

func PreRunSessionToken() {
  token, err := GetSessionToken()
  if err != nil {
    os.Exit(1)
  }

  if token == "" {
    token, err = CreateSessionToken()
  }
  if err != nil {
    os.Exit(1)
  }

  sessionToken = token
  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()
}

func PreRunVaultUUID(shouldUpsert bool) {
  vaultName = GetVaultName()
  uuid, err := GetVaultUUID(shouldUpsert)
  if (!shouldUpsert && uuid == "") || err != nil {
    os.Exit(1)
  }
  if shouldUpsert && uuid == "" {
    uuid, err = UpsertVaultUUID()
  }
  if uuid == "" {
    os.Exit(1)
  }

  vaultUUID = uuid
  viper.Set(vaultUUIDKey, vaultUUID)
  viper.WriteConfig()
}

// command

func Vault(_ *cobra.Command, args []string) {
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
}

// Get retrieves a credential from 1Password
func Get(_ *cobra.Command, _ []string) {
  bytes, err := OpGet(false, "item", key, "--vault", vaultUUID)
  if err != nil {
    println(err.Error())
    os.Exit(1)
  }
  output := string(bytes)

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
}

// Store upserts a credential in 1Password
func Store(_ *cobra.Command, _ []string) {
  // value := fmt.Sprintf("%s:%s", username, password)
  item, err := OpGet(false, "item", key, "--vault", vaultUUID)
  if err != nil {
    os.Exit(1)
  }

  uuid := gjson.Get(item, "uuid").String()
  storedUsername := gjson.Get(item, "username").String()
  storedPassword := gjson.Get(item, "password").String()

  if username == storedUsername && password == storedPassword {
    os.Exit(1)
  }

  opArgs := []string{fmt.Sprintf("title=%s", key), fmt.Sprintf("username=%s", username), fmt.Sprintf("password=%s", password), "--vault", vaultUUID}

  if uuid == "" {
    _, err = Op(append([]string{"create", "item", "Login"}, opArgs...)...)
  } else {
    _, err = Op(append([]string{"edit", "item", uuid}, opArgs...)...)
  }
  if err != nil {
    os.Exit(1)
  }
}

// Erase removes a credential in 1Password
func Erase(_ *cobra.Command, _ []string) {
  item, err := OpGet(false, "item", key, "--vault", vaultUUID)
  if err != nil {
    os.Exit(1)
  }

  uuid := gjson.Get(item, "uuid").String()

  if item == "" {
    os.Exit(1)
  }

  _, err = Op("delete", "item", uuid, "--vault", vaultUUID)
  if err != nil {
    os.Exit(1)
  }
}

// cached value helpers

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

  bytes, err := OpGet(silent, "vault", vaultName)
  if err != nil {
    return "", err
  }

  return gjson.Get(string(bytes), "uuid").String(), nil
}

// UpsertVaultUUID calls creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func UpsertVaultUUID() (string, error) {
  bytes, err := Op(
    "create",
    "vault", vaultName,
    "--allow-admins-to-manage", "false",
    "--description", fmt.Sprintf("'%s'", vaultDescription),
  )
  if err != nil {
    return "", err
  }

  uuid := gjson.Get(string(bytes), "uuid").String()
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
  bytes, err := Op("signin", "--raw")
  if err != nil {
    println()
    return "", err
  }

  return strings.Trim(string(bytes), whitespace), nil
}

// op helpers

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

  bytes, _ := cmd.CombinedOutput()
  out := strings.Trim(string(bytes), whitespace)

  if !silent && strings.Contains(out, "doesn't seem to be a vault in this account") {
    println(fmt.Sprintf(missinVaultErrMsg, vaultName))
    viper.Set(vaultUUIDKey, "")
    viper.WriteConfig()
  }
  if strings.HasPrefix(out, "[ERROR]") {
    return "", nil
  }
  return out, nil
}

// helpers

// ParseURL reads from stdin and sets key, username and password.
func ParseURL(cmd *cobra.Command) {
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
      rawurl = strings.TrimPrefix(gjson.Get(input, "ServerURL").String(), whitespace)
      username = strings.TrimPrefix(gjson.Get(input, "Username").String(), whitespace)
      password = strings.TrimPrefix(gjson.Get(input, "Secret").String(), whitespace)
    } else {
      rawurl = input
    }
  }

  if rawurl != "" {
    var err error
    URL, err = url.Parse(rawurl)
    if err != nil {
      os.Exit(1)
    }
    if URL.User.Username() != "" {
      username = URL.User.Username()
    }
    URL.User = nil
  }

  key = URL.String()
}
