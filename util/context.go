package util

import (
  "bufio"
  "encoding/json"
  "fmt"
  "net/url"
  "os"
  "path"
  "strings"
  "time"

  "github.com/spf13/cobra"
  "github.com/spf13/viper"
  "github.com/tidwall/gjson"
  "github.com/tlowerison/credential-1password/op"
)

type Mode string

type Context struct {
  ModeFlag          string
  ShouldCreateVault bool
  cmd               *cobra.Command
  configPath        string
  homeDir           string
  input             string
  inputs            map[string]string
  key               string
  mode              Mode
  name              string
  opCtx             *op.Context
  password          string
  username          string
  vaultName         string
  vaultNameDefault  string
}

const (
  DockerMode Mode = "docker"
  GitMode    Mode = "git"
)

var PredefinedModes = []string{
  string(DockerMode),
  string(GitMode),
}

func (m Mode) IsPredefined() bool {
  switch m {
  case DockerMode: break
  case GitMode: break
  default: return false
  }
  return true
}

func (m Mode) Valid() bool {
  return m.IsPredefined() || isValidGenericMode(string(m))
}

const sessionTokenDateKey = "session-token.date"
const sessionTokenValueKey = "session-token.value"

const vaultNameKey = "vault.name"
const vaultUUIDKey = "vault.uuid"
const vaultDescription = "Contains credentials managed by %s."
const vaultNameDefault = "credential-1password"

const dockerServerURLKey = "ServerURL"
const timeFormat = time.UnixDate

const missingVaultErrMsg = "doesn't seem to be a vault in this account"

// Register
func Register(name string) (*Context, error) {
  homeDir, err := os.UserHomeDir()
  if err != nil {
    return nil, err
  }

  configPath := path.Join(homeDir, fmt.Sprintf(".%s", name))
  os.MkdirAll(configPath, 0700)

  viper.AddConfigPath(configPath)
  viper.SetConfigName("config")
  viper.SetConfigType("yaml")

  viper.SetDefault(vaultNameKey, vaultNameDefault)

  _ = viper.SafeWriteConfig()
  err = viper.ReadInConfig()
  if err != nil {
    return nil, err
  }

  return &Context{
    configPath:       configPath,
    homeDir:          homeDir,
    inputs:           map[string]string{},
    name:             name,
    opCtx:            &op.Context{},
    vaultNameDefault: vaultNameDefault,
  }, nil
}

// GetCmd
func (ctx *Context) GetCmd() *cobra.Command {
  return ctx.cmd
}

// GetConfigPath
func (ctx *Context) GetConfigPath() string {
  return ctx.configPath
}

// GetHomeDir
func (ctx *Context) GetHomeDir() string {
  return ctx.homeDir
}

// GetInput
func (ctx *Context) GetInput() string {
  return ctx.input
}

// GetKey
func (ctx *Context) GetKey() (string, error) {
  if ctx.key != "" {
    return ctx.key, nil
  }

  if ctx.input == "" {
    return "", fmt.Errorf("no input has been read")
  }

  modeKey, err := ctx.getModeKey()
  if err != nil {
    return "", err
  }
  if !ctx.GetMode().IsPredefined() {
    return modeKey, nil
  }
  return fmt.Sprintf("%s:%s", string(ctx.GetMode()), modeKey), nil
}

// GetMode
func (ctx *Context) GetMode() Mode {
  if string(ctx.mode) != "" {
    return ctx.mode
  }
  mode := Mode(ctx.ModeFlag)
  if mode.Valid() {
    ctx.mode = mode
  }
  return ctx.mode
}

// GetName
func (ctx *Context) GetName() string {
  mode := ctx.GetMode()
  if mode.IsPredefined() {
    return fmt.Sprintf("%s-%s", string(mode), ctx.name)
  }
  return ctx.name
}

// GetOpQuery
func (ctx *Context) GetOpQuery() (*op.Query, error) {
  key, err := ctx.GetKey()
  if err != nil {
    return nil, err
  }

  opCtx, err := ctx.getOpCtx()
  if err != nil {
    return nil, err
  }

  return &op.Query{Context: *opCtx, Key: key}, nil
}

// GetVaultName
func (ctx *Context) GetVaultName() string {
  if ctx.vaultName == "" {
    vaultNameIntf := viper.Get(vaultNameKey)
    if vaultNameIntf != nil {
      ctx.vaultName = vaultNameIntf.(string)
    }
  }
  return ctx.vaultName
}

// ReadInput scans from stdin and splits each line by "=" to find key/value pairs.
// Any line which does not contain "=" is skipped over. Tries to store the inputs in the provided map,
// but if it's nil, will create a new map and fill that; returns the filled inputs map.
func (ctx *Context) ReadInput() error {
  if ctx.cmd == nil {
    return fmt.Errorf("unable to read inputs in the correct format without knowledge of the current command")
  }

  var lines []string
  if !ctx.GetMode().IsPredefined() && ctx.GetCmd().Use != "store" {
    lines = []string{}
  } else {
    lines = scanStdinLines()
  }
  ctx.input = strings.Join(lines, "\n") + "\n"

  mode := ctx.GetMode()

  switch mode {
  case DockerMode:
    if ctx.cmd.Use == "store" {
      return ctx.readJSONInputs(lines)
    } else {
      return ctx.readServerURLInput(lines)
    }
  case GitMode:
    return ctx.readKeyValueInputs(lines)
  default:
    return nil
  }
}

// SetCmd
func (ctx *Context) SetCmd(cmd *cobra.Command) {
  ctx.cmd = cmd
}

// SetVaultName
func (ctx *Context) SetVaultName(vaultName string, shouldCreate bool) error {
  ctx.vaultName = vaultName
  ctx.setVaultUUID("")

  opCtx, err := ctx.getOpCtx()
  if err != nil {
    return err
  }

  vault, err := op.GetVault(op.Query{Context: *opCtx, Key: vaultName})
  if err != nil {
    return err
  }

  vaultUUID := gjson.Get(vault, "uuid").String()

  if vaultUUID != "" {
    ctx.setVaultName(vaultName)
    ctx.setVaultUUID(vaultUUID)
    return nil
  }
  if !shouldCreate {
    return fmt.Errorf("unable to get specified vault's uuid")
  }

  vaultUUID, err = ctx.createVault(vaultName)
  if err != nil {
    return err
  }

  ctx.setVaultName(vaultName)
  ctx.setVaultUUID(vaultUUID)
  return nil
}

// Signin clears the current cached session token, requests the user to signin,
// stores the new returned session token and returns it as well.
func (ctx *Context) Signin() (string, error) {
  ctx.clearSessionToken()
  sessionToken, err := op.Signin()
  if err != nil {
    return "", err
  }

  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()

  return sessionToken, nil
}


// -- generic helpers --

func isValidGenericMode(mode string) bool {
  if mode == "" || strings.Contains(mode, " \n\t") {
    return false
  }
  for _, predefinedMode := range PredefinedModes {
    if strings.HasPrefix(strings.ToLower(mode), predefinedMode) {
      return false
    }
  }
  return true
}

// scanStdinLines scans stdin until it reads two newlines or EOF,
// closes os.Stdin and returns the scanned lines.
func scanStdinLines() []string {
  scanner := bufio.NewScanner(os.Stdin)
  defer os.Stdin.Close()
  lines := []string{}
	for scanner.Scan() {
    line := scanner.Text()
    lines = append(lines, line)
    if line == "" {
      break
    }
  }
  return lines
}

// scrubURL removes the User and Password fields from the provided url
func scrubURL(URL *url.URL) {
  URL.User = nil
}

// -- ctx helper fns --

// clearSessionToken clears all session token related config values.
func (ctx *Context) clearSessionToken() {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }
  ctx.opCtx.SessionToken = ""
  viper.Set(sessionTokenDateKey, "")
  viper.Set(sessionTokenValueKey, "")
  viper.WriteConfig()
}

// createVault
func (ctx *Context) createVault(vaultName string) (string, error) {
  sessionToken, err := ctx.getSessionToken()
  if err != nil {
    return "", err
  }

  output, err := op.CreateVault(op.CreateVaultMutation{
    SessionToken: sessionToken,
    Title: vaultName,
    Description: fmt.Sprintf(vaultDescription, ctx.GetName()),
  })
  if err != nil {
    return "", err
  }

  vaultUUID := gjson.Get(output, "uuid").String()
  if vaultUUID == "" {
    return "", fmt.Errorf("unable to get specified vault's uuid")
  }

  return vaultUUID, nil
}

// getModeKey
func (ctx *Context) getModeKey() (string, error) {
  mode := ctx.GetMode()
  switch mode {
  case DockerMode:
    return ctx.getDockerKey()
  case GitMode:
    return ctx.getGitKey()
  default:
    if string(mode) == "" {
      return "", fmt.Errorf("unknown mode %s", ctx.ModeFlag)
    }
    return string(mode), nil
  }
}

// getOpCtx gets the configured vault uuid and session token and stores them in context.
func (ctx *Context) getOpCtx() (*op.Context, error) {
  if ctx.opCtx != nil && ctx.opCtx.SessionToken != "" && ctx.opCtx.VaultUUID != "" {
    return ctx.opCtx, nil
  }

  var vaultUUID string
  vaultUUIDIntf := viper.Get(vaultUUIDKey)
  if vaultUUIDIntf != nil {
    vaultUUID = vaultUUIDIntf.(string)
  }

  sessionToken, err := ctx.getSessionToken()
  if err != nil {
    return nil, err
  }

  if vaultUUID != "" && sessionToken != "" {
    ctx.opCtx = &op.Context{
      SessionToken: sessionToken,
      VaultUUID:    vaultUUID,
    }
    return ctx.opCtx, nil
  }

  vaultName := ctx.GetVaultName()

  output, err := op.GetVault(op.Query{
    Context: op.Context{SessionToken: sessionToken},
    Key: vaultName,
  })
  if err != nil {
    return nil, err
  }

  vaultUUID = gjson.Get(output, "uuid").String()
  if vaultUUID == "" {
    if vaultName != ctx.vaultNameDefault {
      return nil, fmt.Errorf("unable to get the uuid of vault named '%s'", vaultName)
    }
    vaultUUID, err = ctx.createVault(vaultName)
    if err != nil {
      return nil, err
    }
  }

  ctx.setVaultUUID(vaultUUID)

  ctx.opCtx = &op.Context{
    SessionToken: sessionToken,
    VaultUUID:    vaultUUID,
  }
  return ctx.opCtx, nil
}

// getSessionToken retrieves the locally stored session token, if still valid.
func (ctx *Context) getSessionToken() (string, error) {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }

  if ctx.opCtx.SessionToken != "" {
    return ctx.opCtx.SessionToken, nil
  }

  dateIntf := viper.Get(sessionTokenDateKey)
  if dateIntf == nil {
    return ctx.Signin()
  }

  date, err := time.Parse(timeFormat, dateIntf.(string))
  if err != nil || time.Now().Sub(date).Minutes() >= 30 {
    return ctx.Signin()
  }

  sessionTokenIntf := viper.Get(sessionTokenValueKey)
  if sessionTokenIntf == nil {
    return ctx.Signin()
  }

  ctx.opCtx.SessionToken = sessionTokenIntf.(string)
  return ctx.opCtx.SessionToken, nil
}

// readJSONInputs
func (ctx *Context) readJSONInputs(lines []string) error {
  input := []byte(strings.Join(lines, "\n"))
  return json.Unmarshal(input, &ctx.inputs)
}

// readKeyValueInputs
func (ctx *Context) readKeyValueInputs(lines []string) error {
  for _, line := range lines {
    elements := strings.Split(line, "=")
    if len(elements) >= 2 {
      ctx.inputs[elements[0]] = strings.Join(elements[1:], "=")
    }
  }
  return nil
}

// readServerURLInput
func (ctx *Context) readServerURLInput(lines []string) error {
  input := strings.TrimSpace(strings.Join(lines, "\n"))
  if len(strings.Split(input, "\n")) != 1 {
    return fmt.Errorf("cannot parse url from multiple lines of input")
  }

  ctx.inputs[dockerServerURLKey] = lines[0]
  return nil
}

// setVaultName
func (ctx *Context) setVaultName(vaultName string) {
  ctx.vaultName = vaultName
  viper.Set(vaultNameKey, vaultName)
  viper.WriteConfig()
}

// setVaultUUID
func (ctx *Context) setVaultUUID(vaultUUID string) {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }
  ctx.opCtx.VaultUUID = vaultUUID
  viper.Set(vaultUUIDKey, vaultUUID)
  viper.WriteConfig()
}

// -- mode specific fns --

// getDockerKey
func (ctx *Context) getDockerKey() (string, error) {
  cmd := ctx.GetCmd()
  if cmd == nil {
    return "", fmt.Errorf("cannot get docker key: unable to determine how to read inputs without knowledge of what command was run")
  }

  URL, err := url.Parse(ctx.inputs[dockerServerURLKey])
  if err != nil {
    return "", err
  }

  scrubURL(URL)
  return URL.String(), nil
}

// getGitKey
func (ctx *Context) getGitKey() (string, error) {
  URL := &url.URL{}

  rawurl, hasRawurl := ctx.inputs["url"]
  if hasRawurl {
    var err error
    URL, err = url.Parse(rawurl)
    if err != nil {
      return "", err
    }
  } else {
    host, hasHost := ctx.inputs["host"]
    if hasHost {
      URL.Host = host
    } else {
      return "", fmt.Errorf("host is missing in credentials")
    }

    scheme, hasScheme := ctx.inputs["protocol"]
    if hasScheme {
      URL.Scheme = scheme
    } else {
      return "", fmt.Errorf("protocol is missing in credentials")
    }

    path, hasPath := ctx.inputs["path"]
    if hasPath {
      URL.Path = path
    }
  }

  scrubURL(URL)

  return URL.String(), nil
}
