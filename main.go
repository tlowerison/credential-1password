package main

import (
	"fmt"
  "os"
	"os/exec"
  "path"
  "strings"
	"time"

	"github.com/spf13/cobra"
  "github.com/spf13/viper"
	"github.com/tidwall/gjson"
)

const app = "git-credential-1password"

// util

const whitespace = " \n\t"

// config

var configPath = ""
const timeFormat = time.UnixDate

var sessionToken string
const sessionTokenKey = "session-token"

var sessionTokenDateKey = fmt.Sprintf("%s.date", sessionTokenKey)
var sessionTokenValueKey = fmt.Sprintf("%s.value", sessionTokenKey)

const vaultKey = "vault"

var vaultName = "git-credentials"
var vaultNameKey = fmt.Sprintf("%s.name", vaultKey)

var vaultUUID string
var vaultUUIDKey = fmt.Sprintf("%s.uuid", vaultKey)

// main

func main() {
  RegisterConfig()

	rootCmd := &cobra.Command{
	  Use:   app,
	  Short: fmt.Sprintf("git-credential helper for 1Password.", app),
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Println(cmd.UsageString())
		},
	}

	rootCmd.AddCommand(&cobra.Command{
	  Use:   "vault",
	  Short: "get/set the vault that git-credential uses",
		Args: cobra.RangeArgs(0, 1),
		Run: Vault,
	})

	rootCmd.AddCommand(&cobra.Command{
	  Use:   "get",
	  Short: "retrieve credential by key",
		Args: cobra.ExactArgs(1),
		PreRun: PreRun,
		Run: Get,
	})

	rootCmd.AddCommand(&cobra.Command{
	  Use:   "store",
	  Short: "store key/credential pair",
		Args: cobra.ExactArgs(2),
		PreRun: PreRun,
		Run: Store,
	})

	rootCmd.AddCommand(&cobra.Command{
	  Use:   "erase",
	  Short: "erase credential by key",
		Args: cobra.ExactArgs(1),
		PreRun: PreRun,
		Run: Erase,
	})

	rootCmd.Execute()
}

func RegisterConfig() {
  homeDir, err := os.UserHomeDir()
  if err != nil {
    panic(err)
  }

  configPath = path.Join(homeDir, fmt.Sprintf(".%s", app))
  os.MkdirAll(configPath, 0700)

  viper.AddConfigPath(configPath)
  viper.SetConfigName("config")
  viper.SetConfigType("yaml")

	viper.SetDefault(vaultNameKey, vaultName)

  viper.SafeWriteConfig()
  err = viper.ReadInConfig()
  if err != nil {
  	panic(fmt.Errorf("Fatal error config file: %s \n", err))
  }
}

// pre-run

func PreRun(_ *cobra.Command, _ []string) {
	PreRunSessionToken()
	PreRunVaultUUID()
	viper.WriteConfig()
}

func PreRunSessionToken() {
	token, err := GetSessionToken()
	if err != nil {
		Exit("prerun failed: session token", err)
	}

	if token == "" {
		token, err = CreateSessionToken()
	}
	if err != nil {
		Exit("prerun failed: session token", err)
	}

	sessionToken = token
	viper.Set(sessionTokenDateKey, time.Now().Format(timeFormat))
  viper.Set(sessionTokenValueKey, sessionToken)
}

func PreRunVaultUUID() {
	vaultName = GetVaultName()
	uuid, err := GetVaultUUID()
	if err != nil {
		Exit("prerun failed: vault uuid", err)
	}

	if uuid == "" {
		uuid, err = CreateVaultUUID()
	}
	if err != nil {
		Exit("prerun failed: vault uuid", err)
	}

	vaultUUID = uuid
  viper.Set(vaultUUIDKey, vaultUUID)
}

// command

func Vault(_ *cobra.Command, args []string) {
	if len(args) == 0 {
		fmt.Println(GetVaultName())
	} else {
		vaultName = args[0]
		viper.Set(vaultNameKey, vaultName)
		viper.Set(vaultUUIDKey, "")
		viper.WriteConfig()
	}
}

// get retrieves a credential from 1Password
func Get(_ *cobra.Command, args []string) {
	key := args[0]
	bytes, err := OpGet("item", key, "--vault", vaultUUID)
	if err != nil {
		Exit("unable to get credential", err)
	}

	credential := gjson.Get(string(bytes), "details.password").String()
	if credential != "" {
		fmt.Println(credential)
	}
}

// store upserts a credential in 1Password
func Store(_ *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	item, err := OpGet("item", key, "--vault", vaultUUID)
	if err != nil {
		Exit("unable to store credential", err)
	}

	uuid := gjson.Get(item, "uuid").String()
	credential := gjson.Get(item, "password").String()

	if credential == value {
		os.Exit(0)
	}

	opArgs := []string{fmt.Sprintf("title=%s", key), fmt.Sprintf("password=%s", value), "--vault", vaultUUID}

	if uuid == "" {
		_, err = Op(append([]string{"create", "item", "Password"}, opArgs...)...)
	} else {
		_, err = Op(append([]string{"edit", "item", uuid}, opArgs...)...)
	}
	if err != nil {
		Exit("unable to store credential", err)
	}
}

// erase removes a credential in 1Password
func Erase(_ *cobra.Command, args []string) {
	key := args[0]

	item, err := OpGet("item", key, "--vault", vaultUUID)
	if err != nil {
		Exit("unable to erase credential", err)
	}

	uuid := gjson.Get(item, "uuid").String()

	if item == "" {
		os.Exit(0)
	}

	_, err = Op("delete", "item", uuid, "--vault", vaultUUID)
	if err != nil {
		Exit("unable to erase credential", err)
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

// getVaultUUID gets the configured vault's uuid.
func GetVaultUUID() (string, error) {
	vaultUUIDIntf := viper.Get(vaultUUIDKey)
	if vaultUUIDIntf != nil {
		uuid := vaultUUIDIntf.(string)
		if uuid != "" {
			return uuid, nil
		}
	}

	bytes, err := OpGet("vault", vaultName)
	if err != nil {
		return "", err
	}

	return gjson.Get(string(bytes), "uuid").String(), nil
}

// createVaultUUID calls creates a new 1Passord vault and returns
// the newly created vault's uuid on success.
func CreateVaultUUID() (string, error) {
	bytes, err := Op(
		"create",
		"vault", vaultName,
		"--allow-admins-to-manage", "false",
		"--description", "'Contains credentials managed by git-credential-1password.'",
	)
	if err != nil {
		return "", err
	}

	uuid := gjson.Get(string(bytes), "uuid").String()
	return uuid, nil
}

// getSessionToken retrieves the locally stored session token, if still valid.
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

// createSessionToken requests the user to sign into 1Password through
// stdin, then stores the provided session token in the local config.
func CreateSessionToken() (string, error) {
	bytes, err := Op("signin", "my", "--raw")
	if err != nil {
		return "", err
	}

	return strings.Trim(string(bytes), whitespace), nil
}

// `op` helpers

// op wraps 1Password's cli tool `op`.
func Op(args ...string) ([]byte, error) {
	cmd := exec.Command("op", append(args, "--session", sessionToken)...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
 	return cmd.Output()
}

// opGet wraps `op get` with hidden stderr and missing resources
// returned as an empty json object.
func OpGet(args ...string) (string, error) {
	cmd := exec.Command("op", append(append([]string{"get"}, args...), "--session", sessionToken)...)
	cmd.Stdin = os.Stdin

 	bytes, _ := cmd.Output()
	out := string(bytes)

	if strings.HasPrefix(strings.TrimPrefix(out, whitespace), "[ERROR]") {
		return "", nil
	}
	return out, nil
}

// helpers

// exit takes a msg and optional list of errors, prints them
// and exits with status code 1.
func Exit(msg string, errs ...error) {
	if len(errs) > 0 && errs[0] != nil {
		fmt.Printf("%s: %s\n", msg, errs[0].Error())
	} else {
		fmt.Println(msg)
	}
	os.Exit(1)
}
