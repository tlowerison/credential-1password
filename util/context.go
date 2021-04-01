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
  Mode              string
  ShouldCreateVault bool
  cmd               *cobra.Command
  configPath        string
  homeDir           string
  key               string
  inputs            map[string]string
  mode              Mode
  name              string
  password          string
  sessionToken      string
  username          string
  vaultName         string
  vaultNameDefault  string
  vaultUUID         string
}

const (
  GitMode    Mode = "git"
  DockerMode Mode = "docker"
  NPMMode    Mode = "npm"
)

const sessionTokenDateKey = "session-token.date"
const sessionTokenValueKey = "session-token.value"

const vaultNameKey = "vault.name"
const vaultUUIDKey = "vault.uuid"
const vaultDescription = "Contains credentials managed by %s."
const vaultNameDefault = "credential-1password"

const dockerServerURLKey = "ServerURL"
const timeFormat = time.UnixDate

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
    name:             name,
    vaultNameDefault: vaultNameDefault,
  }, nil
}

// GetCmd
func (ctx *Context) GetCmd() *cobra.Command {
  return ctx.cmd
}

// SetCmd
func (ctx *Context) SetCmd(cmd *cobra.Command) {
  ctx.cmd = cmd
}

// GetConfigPath
func (ctx *Context) GetConfigPath() string {
  return ctx.configPath
}

// GetHomeDir
func (ctx *Context) GetHomeDir() string {
  return ctx.homeDir
}

// GetInputs
func (ctx *Context) GetInputs() map[string]string {
  if ctx.inputs == nil {
    ctx.inputs = map[string]string{}
  }
  return ctx.inputs
}

// ReadInput scans from stdin and splits each line by "=" to find key/value pairs.
// Any line which does not contain "=" is skipped over. Tries to store the inputs in the provided map,
// but if it's nil, will create a new map and fill that; returns the filled inputs map.
func (ctx *Context) ReadInputs() error {
  if ctx.cmd == nil {
    return fmt.Errorf("unable to read inputs in the correct format without knowledge of the current command")
  }

  if ctx.inputs == nil {
    ctx.inputs = map[string]string{}
  }

  lines := scanStdinLines()
  mode := ctx.GetMode()

  switch mode {
  case GitMode:
    return ctx.readKeyValueInputs(lines)
  case DockerMode:
    if ctx.cmd.Use == "store" {
      return ctx.readJSONInputs(lines)
    } else {
      return ctx.readServerURLInput(lines)
    }
  case NPMMode:
    return ctx.readKeyValueInputs(lines)
  default:
    return fmt.Errorf("could not read inputs for unknown mode %s", ctx.Mode)
  }
}

// GetKey
func (ctx *Context) GetKey() (string, error) {
  if ctx.key != "" {
    return ctx.key, nil
  }

  if ctx.inputs == nil {
    return "", fmt.Errorf("no inputs have been read")
  }

  mode := ctx.GetMode()

  switch mode {
  case GitMode:
    return ctx.getGitKey()
  case DockerMode:
    return ctx.getDockerKey()
  case NPMMode:
    return ctx.getNPMKey()
  default:
    return "", fmt.Errorf("unknown mode %s", ctx.Mode)
  }
}

// GetMode
func (ctx *Context) GetMode() Mode {
  ctx.mode = Mode(ctx.Mode)
  return ctx.mode
}

// GetName
func (ctx *Context) GetName() string {
  return ctx.name
}

// GetPassword
func (ctx *Context) GetPassword() (string, error) {
  if ctx.password != "" {
    return ctx.key, nil
  }

  if ctx.inputs == nil {
    return "", fmt.Errorf("no inputs have been read")
  }

  mode := ctx.GetMode()

  switch mode {
  case GitMode:
    return ctx.getGitPassword()
  case DockerMode:
    return ctx.getDockerPassword()
  case NPMMode:
    return ctx.getNPMPassword()
  default:
    return "", fmt.Errorf("unknown mode %s", ctx.Mode)
  }
}

// GetUsername
func (ctx *Context) GetUsername() (string, error) {
  if ctx.username != "" {
    return ctx.key, nil
  }

  if ctx.inputs == nil {
    return "", fmt.Errorf("no inputs have been read")
  }

  mode := ctx.GetMode()

  switch mode {
  case GitMode:
    return ctx.getGitUsername()
  case DockerMode:
    return ctx.getDockerUsername()
  case NPMMode:
    return ctx.getNPMUsername()
  default:
    return "", fmt.Errorf("unknown mode %s", ctx.Mode)
  }
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

// SetVaultName
func (ctx *Context) SetVaultName(vaultName string, shouldCreate bool) error {
  ctx.vaultName = vaultName

  ctx.setVaultUUID("")
  vaultUUID, err := ctx.GetVaultUUID()
  if err != nil {
    return err
  }

  if vaultUUID != "" {
    return nil
  }
  if !shouldCreate {
    return fmt.Errorf("unable to get specified vault's uuid")
  }

  _, err = ctx.createVault(vaultName)
  if err != nil {
    return err
  }

  ctx.setVaultName(vaultName)
  return nil
}

// GetVaultUUID gets the configured vault's uuid.
func (ctx *Context) GetVaultUUID() (string, error) {
  if ctx.vaultUUID != "" {
    return ctx.vaultUUID, nil
  }

  vaultUUIDIntf := viper.Get(vaultUUIDKey)
  if vaultUUIDIntf != nil {
    vaultUUID := vaultUUIDIntf.(string)
    if vaultUUID != "" {
      ctx.vaultUUID = vaultUUID
      return ctx.vaultUUID, nil
    }
  }

  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return "", err
  }

  vaultName := ctx.GetVaultName()

  outBytes, err := op.Get(sessionToken, true, "vault", vaultName)
  if err != nil {
    return "", err
  }

  vaultUUID := gjson.Get(string(outBytes), "uuid").String()
  if vaultUUID == "" {
    if vaultName != ctx.vaultNameDefault {
      return "", fmt.Errorf("unable to get the uuid of vault named '%s'", vaultName)
    } else {
      vaultUUID, err = ctx.createVault(vaultName)
    }
  }

  ctx.setVaultUUID(vaultUUID)
  return vaultUUID, nil
}

// GetSessionToken retrieves the locally stored session token, if still valid.
func (ctx *Context) GetSessionToken() (string, error) {
  if ctx.sessionToken != "" {
    return ctx.sessionToken, nil
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

  ctx.sessionToken = sessionTokenIntf.(string)
  return ctx.sessionToken, nil
}

// ClearSessionToken clears all session token related config values.
func (ctx *Context) ClearSessionToken() {
  ctx.sessionToken = ""
  viper.Set(sessionTokenDateKey, "")
  viper.Set(sessionTokenValueKey, "")
  viper.WriteConfig()
}

// Signin clears the current cached session token, requests the user to signin,
// stores the new returned session token and returns it as well.
func (ctx *Context) Signin() (string, error) {
  ctx.ClearSessionToken()
  sessionToken, err := op.Signin()
  if err != nil {
    return "", err
  }

  viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
  viper.WriteConfig()

  return sessionToken, nil
}

// - unexported fns -

// -- generic helpers --

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

// createVault
func (ctx *Context) createVault(vaultName string) (string, error) {
  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return "", err
  }

  vaultUUID, err := op.CreateVault(sessionToken, ctx.GetVaultName(), fmt.Sprintf(vaultDescription, ctx.GetName()))
  if err != nil {
    return "", err
  }
  if vaultUUID == "" {
    return "", fmt.Errorf("unable to get specified vault's uuid")
  }

  return vaultUUID, nil
}

// readJSONInputs
func (ctx *Context) readJSONInputs(lines []string) error {
  var inputs map[string]string

  input := []byte(strings.Join(lines, "\n"))
  err := json.Unmarshal(input, &inputs)
  if err != nil {
    return err
  }

  ctx.inputs = inputs
  return nil
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

  ctx.inputs = map[string]string{dockerServerURLKey: lines[0]}
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
  ctx.vaultUUID = vaultUUID
  viper.Set(vaultUUIDKey, vaultUUID)
  viper.WriteConfig()
}

// -- mode specific fns --

// --- git ---

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

// getGitPassword
func (ctx *Context) getGitPassword() (string, error) {
  if rawurl, hasRawurl := ctx.inputs["url"]; hasRawurl {
    URL, err := url.Parse(rawurl)
    if err != nil {
      return "", err
    }

    if URL.User == nil {
      return "", fmt.Errorf("url does not contain password")
    }

    password, hasPassword := URL.User.Password()
    if !hasPassword {
      return "", fmt.Errorf("url does not contain password")
    }
    return password, nil
  }

  if password, hasPassword := ctx.inputs["password"]; hasPassword {
    return password, nil
  } else {
    return "", fmt.Errorf("password is missing in credentials")
  }
}

// getGitUsername
func (ctx *Context) getGitUsername() (string, error) {
  if rawurl, hasRawurl := ctx.inputs["url"]; hasRawurl {
    URL, err := url.Parse(rawurl)
    if err != nil {
      return "", err
    }

    if URL.User == nil {
      return "", fmt.Errorf("url does not contain password")
    }

    return URL.User.Username(), nil
  }

  if username, hasUsername := ctx.inputs["username"]; hasUsername {
    return username, nil
  } else {
    return "", fmt.Errorf("username is missing in credentials")
  }
}

// --- docker ---

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

// getDockerPassword
func (ctx *Context) getDockerPassword() (string, error) {
  cmd := ctx.GetCmd()
  if cmd == nil {
    return "", fmt.Errorf("cannot get docker password: unable to determine how to read inputs without knowledge of what command was run")
  }

  if password, hasPassword := ctx.inputs["Secret"]; hasPassword {
    return password, nil
  } else {
    return "", fmt.Errorf("Secret is missing in credentials")
  }
}

// getDockerUsername
func (ctx *Context) getDockerUsername() (string, error) {
  if username, hasUsername := ctx.inputs["Username"]; hasUsername {
    return username, nil
  } else {
    return "", fmt.Errorf("Username is missing in credentials")
  }
}

// --- npm ---

// getNPMKey
func (ctx *Context) getNPMKey() (string, error) {
  registry, hasRegistry := ctx.inputs["registry"]
  if !hasRegistry {
    return "", fmt.Errorf("registry is missing npm credentials")
  }

  URL, err := url.Parse(registry)
  if err != nil {
    return "", err
  }

  scrubURL(URL)
  return URL.String(), nil
}

// getNPMPassword
func (ctx *Context) getNPMPassword() (string, error) {
  if password, hasPassword := ctx.inputs["_auth"]; hasPassword {
    return password, nil
  } else {
    return "", fmt.Errorf("_auth is missing in credentials")
  }
}

// getNPMUsername
func (ctx *Context) getNPMUsername() (string, error) {
  if username, hasUsername := ctx.inputs["email"]; hasUsername {
    return username, nil
  } else {
    return "", fmt.Errorf("email is missing in credentials")
  }
}
