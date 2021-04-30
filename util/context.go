package util

import (
  "bufio"
  "encoding/json"
  "fmt"
  "io"
  "net/url"
  "strings"
  "time"

  "github.com/spf13/cobra"
  "github.com/tidwall/gjson"
  "github.com/tlowerison/credential-1password/keystore"
  "github.com/tlowerison/credential-1password/op"
)

type Mode string

type Flags struct {
  Mode                string
  Config_Vault_Create bool
}

type Context struct {
  Flags       *Flags
  cmd         *cobra.Command
  input       string
  inputs      map[string]string
  key         string
  keystore    keystore.Keystore
  mode        Mode
  name        string
  OpFunc      op.OpFunc
  opCtx       *op.Context
  password    string
  serviceName string
  stdin       io.ReadCloser
  stdinDeadline time.Duration
  username    string
  vaultName   string
}

const (
  DockerMode Mode = "docker"
  GitMode    Mode = "git"
)

var PredefinedModes = []string{
  string(DockerMode),
  string(GitMode),
}

const VaultKey = "vault"

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

const ErrMsgUnknownCommand = "unable to read inputs in the correct format without knowledge of the current command"
const ErrMsgDockerServerUrlBadInputZeroLines = "cannot parse url from zero lines of input"
const ErrMsgDockerServerUrlBadInputMultipleLines = "cannot parse url from multiple lines of input"
const ErrMsgClosedStdinAfterDeadline = "closed stdin after waiting"
const ServiceName = "com.tlowerison.credential-1password"

const sessionTokenDateKey = "session-token.date"
const sessionTokenValueKey = "session-token.value"

var vaultNameKey = fmt.Sprintf("%s.name", VaultKey)
var vaultUUIDKey = fmt.Sprintf("%s.uuid", VaultKey)
const vaultDescription = "Contains credentials managed by %s."
const vaultNameDefault = "credential-1password"

const dockerServerURLKey = "ServerURL"
const timeFormat = time.UnixDate
const defaultStdinDeadline = 30 * time.Second

func NewContext(opFunc op.OpFunc, ks keystore.Keystore, stdin io.ReadCloser) *Context {
  return &Context{
    Flags:        &Flags{},
    OpFunc:       opFunc,
    opCtx:        &op.Context{},
    inputs:       map[string]string{},
    keystore:     ks,
    stdin:        stdin,
    stdinDeadline: defaultStdinDeadline,
  }
}

// GetCmd returns the private cmd field.
func (ctx *Context) GetCmd() *cobra.Command {
  return ctx.cmd
}

// GetInput returns the private field input.
// input is the cached value of what is read
// from stdin.
func (ctx *Context) GetInput() string {
  return ctx.input
}

// GetInputs is only required for testing, it duplicates
// all inputs in case of any bad usage in order to ensure
// that ctx.inputs is not mutated outside of this package.
func (ctx *Context) GetInputs() map[string]string {
  inputs := map[string]string{}
  if ctx.inputs == nil {
    return inputs
  }
  for key, value := range ctx.inputs {
    inputs[key] = value
  }
  return inputs
}

// GetKey returns the input key provided over stdin.
// This key will be used as the stored file's title.
// If it has already been computed, returns the cached
// value, otherwise, computes it based on whether the
// current mode is predefined or not. If predefined,
// the current mode is assumed to have its own method
// for processing stdin into a key. If not predefined,
// the current mode is returned as the key itself.
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

// GetMode returns the mode set by the persistent flag --mode.
func (ctx *Context) GetMode() Mode {
  if string(ctx.mode) != "" {
    return ctx.mode
  }
  mode := Mode(ctx.Flags.Mode)
  if mode.Valid() {
    ctx.mode = mode
  }
  return ctx.mode
}

// GetName returns "$mode-credential-1password" if using a predefined
// mode, otherwise returns "credential-1password".
func (ctx *Context) GetName() string {
  mode := ctx.GetMode()
  if mode.IsPredefined() {
    return fmt.Sprintf("%s-%s", string(mode), "credential-1password")
  }
  return "credential-1password"
}

// GetOpQuery wraps an op.Context and the input key provided over
// stdin into an op.Query. This is a useful helper function as
// op.Query is embedded into most op structs.
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

// GetSessionToken retrieves the session token in context if present. If not,
// checks if the most recent session token date if it's still valid, and if it
// is, tries to return whatever is stored in the encrypted keystore. If there's
// nothing in the keystore or the token is out of date, it will request the user
// to sigin, store the newly created session token in the encrypted keystore
// as well as context, and return the session token.
func (ctx *Context) GetSessionToken() (string, error) {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }

  if ctx.opCtx.SessionToken != "" {
    return ctx.opCtx.SessionToken, nil
  }

  sessionTokenDate, err := ctx.keystore.Get(sessionTokenDateKey)
  if err != nil || sessionTokenDate == "" {
    return ctx.Signin()
  }

  date, err := time.Parse(timeFormat, sessionTokenDate)
  if err != nil || time.Now().Sub(date).Minutes() >= 30 {
    return ctx.Signin()
  }

  sessionToken, err := ctx.keystore.Get(sessionTokenValueKey)

  if err != nil {
    return "", err
  } else if sessionToken == "" {
    return ctx.Signin()
  }

  ctx.opCtx.SessionToken = sessionToken
  return ctx.opCtx.SessionToken, nil
}

// GetStdinDeadline returns the stdin timeout.
func (ctx *Context) GetStdinDeadline() time.Duration {
  return ctx.stdinDeadline
}

// GetVaultName reads the configured vault name
// or returns the cached value if already read.
func (ctx *Context) GetVaultName() (string, error) {
  if ctx.vaultName != "" {
    return ctx.vaultName, nil
  }

  vaultName, err := ctx.keystore.Get(vaultNameKey)
  if err != nil {
    return "", err
  }

  if vaultName != "" {
    ctx.vaultName = vaultName
    return vaultName, nil
  }

  ctx.vaultName = vaultNameDefault
  return vaultNameDefault, ctx.keystore.Set(vaultNameKey, vaultNameDefault)
}

// ParseInput scans from stdin and splits each line by "=" to find key/value pairs.
// Any line which does not contain "=" is skipped over. Tries to store the inputs in the provided map,
// but if it's nil, will create a new map and fill that; returns the filled inputs map.
func (ctx *Context) ParseInput() (err error) {
  if ctx.cmd == nil {
    return fmt.Errorf(ErrMsgUnknownCommand)
  }

  cmdName := strings.Split(ctx.GetCmd().Use, " ")[0]
  var lines []string
  if !ctx.GetMode().IsPredefined() && cmdName != "store" {
    lines = []string{}
  } else {
    lines, err = ctx.scanStdinLines()
    if err != nil {
      return err
    }
  }
  ctx.input = strings.Join(lines, "\n") + "\n"

  mode := ctx.GetMode()

  switch mode {
  case DockerMode:
    if cmdName == "store" {
      return ctx.parseJSONInputs(lines)
    } else {
      return ctx.parseServerURLInput(lines)
    }
  case GitMode:
    return ctx.parseKeyValueInputs(lines)
  default:
    return nil
  }
}

// SetCmd sets the private cmd field.
// cmd should be assigned by a prerun cobra command hook.
func (ctx *Context) SetCmd(cmd *cobra.Command) {
  ctx.cmd = cmd
}

// SetStdinDeadline sets the private stdinDeadline field.
func (ctx *Context) SetStdinDeadline(stdinDeadline time.Duration) {
  ctx.stdinDeadline = stdinDeadline
}

// SetVaultName does:
// 1. checks whether the provided vault exists
// 2a. if so, sets the vault name and the vaultUUID in the encrypted keystore
// 2b. if not, and shouldCreate is false, fails
// 2c. if not, and shouldCreate is true, creates a new vault with the provided name, loop back to step 2a
func (ctx *Context) SetVaultName(vaultName string, shouldCreate bool) error {
  ctx.vaultName = vaultName
  ctx.setVaultUUID("")

  opCtx, err := ctx.getOpCtx()
  if err != nil {
    return err
  }

  vault, err := op.GetVault(ctx.OpFunc, op.Query{Context: *opCtx, Key: vaultName})
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
  sessionToken, err := op.Signin(ctx.OpFunc)
  if err != nil {
    return "", err
  }

  err = ctx.setSessionToken(sessionToken)
  if err != nil {
    return "", err
  }

  return sessionToken, nil
}


// --- ctx helper fns ---

// clearSessionToken clears all session token related config values.
func (ctx *Context) clearSessionToken() {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }
  ctx.opCtx.SessionToken = ""
  ctx.keystore.Set(sessionTokenDateKey, "")
  ctx.keystore.Set(sessionTokenValueKey, "")
}

// createVault gets a session token, attempts to create a 1Password vault
// with the op utils, and if successful, returns the created vault's uuid.
func (ctx *Context) createVault(vaultName string) (string, error) {
  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return "", err
  }

  output, err := op.CreateVault(ctx.OpFunc, op.CreateVaultMutation{
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

// getModeKey muxes different modes to derive a mode specific key from stdin input.
func (ctx *Context) getModeKey() (string, error) {
  mode := ctx.GetMode()
  switch mode {
  case DockerMode:
    return ctx.getDockerKey()
  case GitMode:
    return ctx.getGitKey()
  default:
    if string(mode) == "" {
      return "", fmt.Errorf("unknown mode %s", ctx.Flags.Mode)
    }
    return string(mode), nil
  }
}

// getOpCtx gets the configured vault uuid and session token and stores them in context.
func (ctx *Context) getOpCtx() (*op.Context, error) {
  if ctx.opCtx != nil && ctx.opCtx.SessionToken != "" && ctx.opCtx.VaultUUID != "" {
    return ctx.opCtx, nil
  }

  sessionToken, err := ctx.GetSessionToken()
  if err != nil {
    return nil, err
  }

  vaultUUID, err := ctx.getVaultUUID()
  if err != nil {
    return nil, err
  }

  if vaultUUID != "" && sessionToken != "" {
    ctx.opCtx = &op.Context{
      SessionToken: sessionToken,
      VaultUUID:        vaultUUID,
    }
    return ctx.opCtx, nil
  }

  vaultName, err := ctx.GetVaultName()
  if err != nil {
    return nil, err
  }

  output, err := op.GetVault(ctx.OpFunc, op.Query{
    Context: op.Context{SessionToken: sessionToken},
    Key: vaultName,
  })
  if err != nil {
    return nil, err
  }

  vaultUUID = gjson.Get(output, "uuid").String()
  if vaultUUID == "" {
    if vaultName != vaultNameDefault {
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
    VaultUUID:        vaultUUID,
  }
  return ctx.opCtx, nil
}

// getVaultUUID retrieves the vault uuid in context if present. If not, returns
// whatever is currently stored in the encrypted keystore for the vault uuid.
func (ctx *Context) getVaultUUID() (string, error) {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }

  if ctx.opCtx.VaultUUID != "" {
    return ctx.opCtx.VaultUUID, nil
  }

  return ctx.keystore.Get(vaultUUIDKey)
}

// parseJSONInputs unmarshals as json the provided scanned lines into ctx.inputs.
func (ctx *Context) parseJSONInputs(lines []string) error {
  input := []byte(strings.Join(lines, "\n"))
  return json.Unmarshal(input, &ctx.inputs)
}

// parseKeyValueInputs processes the provided scanned lines as key=value pairs into ctx.inputs.
func (ctx *Context) parseKeyValueInputs(lines []string) error {
  for _, line := range lines {
    elements := strings.Split(line, "=")
    if len(elements) >= 2 {
      ctx.inputs[elements[0]] = strings.Join(elements[1:], "=")
    }
  }
  return nil
}

// parseServerURLInput processes the provided scanned lines as a single url into a specific key in ctx.inputs.
func (ctx *Context) parseServerURLInput(lines []string) error {
  input := strings.TrimSpace(strings.Join(lines, "\n"))
  if len(lines) == 0 {
    return fmt.Errorf(ErrMsgDockerServerUrlBadInputZeroLines)
  }
  if len(strings.Split(input, "\n")) != 1 {
    return fmt.Errorf(ErrMsgDockerServerUrlBadInputMultipleLines)
  }

  ctx.inputs[dockerServerURLKey] = input
  return nil
}

// setSessionToken sets the provided session token in context and in the encrypted keystore.
func (ctx *Context) setSessionToken(sessionToken string) error {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }
  ctx.opCtx.SessionToken = sessionToken

  err := ctx.keystore.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  if err != nil {
    return err
  }
  return ctx.keystore.Set(sessionTokenValueKey, sessionToken)
}

// setVaultName sets the provided vault name in context and in the encrypted keystore.
func (ctx *Context) setVaultName(vaultName string) {
  ctx.vaultName = vaultName
  ctx.keystore.Set(vaultNameKey, vaultName)
}

// setVaultUUID sets the provided vault uuid in context and in the encrypted keystore.
func (ctx *Context) setVaultUUID(vaultUUID string) {
  if ctx.opCtx == nil {
    ctx.opCtx = &op.Context{}
  }
  ctx.opCtx.VaultUUID = vaultUUID
  ctx.keystore.Set(vaultUUIDKey, vaultUUID)
}


// --- mode specific fns ---

// getDockerKey processes the parsed input from stdin into a url which will
// be used as the title for the stored document in 1Password. The expected
// input format for `get/erase` is a plain url, and the expected input
// format for `store` is json with a top level key "ServerURL".
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

// getGitKey processes the parsed input from stdin into a url which will
// be used as the title for the stored document in 1Password. The expected
// input format for any of `get/store/erase` is multiple lines of key=value
// pairs including `protocol=...` and `host=...`
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

// scanStdinLines scans stdin until it reads two newlines or EOF,
// closes os.Stdin and returns the scanned lines.
func (ctx *Context) scanStdinLines() ([]string, error) {
  scanner := bufio.NewScanner(ctx.stdin)
  defer ctx.stdin.Close()

  c := make(chan []string, 1)
  go func() {
    lines := []string{}
    for scanner.Scan() {
      line := scanner.Text()
      lines = append(lines, line)
      if line == "" {
        break
      }
    }
    c <- lines
  }()

  select {
  case lines := <- c:
    return lines, nil
  case <-time.After(ctx.stdinDeadline):
    return nil, fmt.Errorf("%s %v", ErrMsgClosedStdinAfterDeadline, ctx.stdinDeadline)
  }
}


// --- generic helpers ---

// isValidGenericMode checks whether the mode does
//  have as a prefix any of the predefined modes.
func isValidGenericMode(mode string) bool {
  if mode == "" || strings.Contains(mode, " ") || strings.Contains(mode, "\n") || strings.Contains(mode, "\t") {
    return false
  }
  for _, predefinedMode := range PredefinedModes {
    if strings.HasPrefix(strings.ToLower(mode), predefinedMode) {
      return false
    }
  }
  return true
}

// scrubURL removes the User and Password fields from the provided url
func scrubURL(URL *url.URL) {
  URL.User = nil
}
