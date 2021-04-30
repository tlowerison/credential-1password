package test

import (
  "fmt"
  "io"
  "os"
  "strings"
  "testing"
  "time"

  "github.com/spf13/cobra"
  "github.com/stretchr/testify/require"
  "github.com/tlowerison/credential-1password/keystore"
  "github.com/tlowerison/credential-1password/util"
)

const serviceName = "credential-1password"

type testStdin struct {
  io.ReadCloser
  sleep time.Duration
}

func newTestStdin(content string) *testStdin {
  return &testStdin{
    ReadCloser: io.NopCloser(strings.NewReader(content)),
    sleep:      0 * time.Millisecond,
  }
}

func (stdin *testStdin) Read(p []byte) (n int, err error) {
  time.Sleep(stdin.sleep)
  return stdin.ReadCloser.Read(p)
}

func (stdin *testStdin) Close() error {
  return stdin.ReadCloser.Close()
}


func testOpFunc(stdin string, args []string) (string, error) {
  return "", nil
}

func TestModeIsPredefined(t *testing.T) {
  require.True(t, util.DockerMode.IsPredefined())
  require.True(t, util.GitMode.IsPredefined())
  require.False(t, util.Mode("npm").IsPredefined())
}

func TestModeValid(t *testing.T) {
  require.True(t, util.DockerMode.Valid())
  require.True(t, util.GitMode.Valid())
  require.True(t, util.Mode("npm").Valid())
  require.False(t, util.Mode(" npm").Valid())
  require.False(t, util.Mode("npm ").Valid())
  require.False(t, util.Mode("np m").Valid())
  require.False(t, util.Mode("\nnpm").Valid())
  require.False(t, util.Mode("npm\n").Valid())
  require.False(t, util.Mode("np\nm").Valid())
  require.False(t, util.Mode("\tnpm").Valid())
  require.False(t, util.Mode("npm\t").Valid())
  require.False(t, util.Mode("np\tm").Valid())
  require.False(t, util.Mode("git_").Valid())
  require.False(t, util.Mode("docker_").Valid())
  require.True(t, util.Mode("npm_").Valid())
}

func TestNewContext(t *testing.T) {
  ctx := util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(""))
  require.NotNil(t, ctx)
}

func TestContextCmd(t *testing.T) {
  ctx := util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(""))
  expCmd := &cobra.Command{}

  ctx.SetCmd(expCmd)
  cmd := ctx.GetCmd()
  require.NotNil(t, cmd)
  require.Equal(t, expCmd, cmd)
}

func TestContextInputGet(t *testing.T) {
  input := ""
  ctx := util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)

  err := ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, util.ErrMsgUnknownCommand, err.Error())

  // timeout (Fail)
  stdin := newTestStdin("")
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), stdin)
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.SetStdinDeadline(50 * time.Millisecond)
  ctx.Flags.Mode = string(util.GitMode)
  stdin.sleep = 100 * time.Millisecond

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, fmt.Sprintf("%s %v", util.ErrMsgClosedStdinAfterDeadline, ctx.GetStdinDeadline()), err.Error())

  // timeout (OK)
  stdin = newTestStdin("")
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), stdin)
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.SetStdinDeadline(100 * time.Millisecond)
  ctx.Flags.Mode = string(util.GitMode)
  stdin.sleep = 50 * time.Millisecond

  err = ctx.ParseInput()
  require.Nil(t, err)

  // non-predefined mode get
  input = ""
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs := ctx.GetInputs()
  require.Equal(t, map[string]string{}, inputs)

  // posititional arguments shouldn't obfuscate command name
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get <foo> <bar>"})

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{}, inputs)

  // happy path git-credential-1password get
  input = "protocol=https\nhost=github.com"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.Flags.Mode = string(util.GitMode)

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{
    "protocol": "https",
    "host":     "github.com",
  }, inputs)

  // empty git-credential-1password get (OK)
  input = ""
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.Flags.Mode = string(util.GitMode)

  err = ctx.ParseInput()
  require.Nil(t, err)

  // happy path docker-credential-1password get
  input = "https://index.docker.io/v1/"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{
    "ServerURL": "https://index.docker.io/v1/",
  }, inputs)

  // empty docker-credential-1password get
  input = ""
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, util.ErrMsgDockerServerUrlBadInputZeroLines, err.Error())

  // multiple lines docker-credential-1password get
  input = "https://index.docker.io/v1/\nhttps://index.docker.io/v1/"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "get"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, util.ErrMsgDockerServerUrlBadInputMultipleLines, err.Error())
}

func TestContextInputStore(t *testing.T) {
  input := ""
  ctx := util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)

  err := ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, util.ErrMsgUnknownCommand, err.Error())

  // timeout
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), os.Stdin)
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})
  ctx.SetStdinDeadline(0 * time.Second)

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, fmt.Sprintf("%s %v", util.ErrMsgClosedStdinAfterDeadline, ctx.GetStdinDeadline()), err.Error())

  // non-predefined mode store
  input = "@scope:registry=https://registry.yarnpkg.com/\n_authToken=my-auth-token\nalways-auth=true"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs := ctx.GetInputs()
  require.Equal(t, map[string]string{}, inputs)

  // posititional arguments shouldn't obfuscate command name
  input = "@scope:registry=https://registry.yarnpkg.com/\n_authToken=my-auth-token\nalways-auth=true"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store <foo> <bar>"})

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{}, inputs)

  // happy path git-credential-1password store
  input = "protocol=https\nhost=github.com\nusername=my-username\npassword=my-password"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})
  ctx.Flags.Mode = string(util.GitMode)

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{
    "protocol": "https",
    "host":     "github.com",
    "username": "my-username",
    "password": "my-password",
  }, inputs)

  // happy path docker-credential-1password store
  input = "{\"ServerURL\": \"https://index.docker.io/v1/\",\n \"Username\": \"my-username\",\n \"Secret\": \"my-secret\" }"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.Nil(t, err)
  require.Equal(t, input + "\n", ctx.GetInput())

  inputs = ctx.GetInputs()
  require.Equal(t, map[string]string{
    "ServerURL": "https://index.docker.io/v1/",
    "Username":  "my-username",
    "Secret":    "my-secret",
  }, inputs)

  // empty docker-credential-1password store
  input = ""
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, "unexpected end of JSON input", err.Error())

  // only url docker-credential-1password store
  input = "https://index.docker.io/v1/"
  ctx = util.NewContext(serviceName, testOpFunc, keystore.NewMockKeystore(nil, nil), newTestStdin(input))
  require.NotNil(t, ctx)
  ctx.SetCmd(&cobra.Command{Use: "store"})
  ctx.Flags.Mode = string(util.DockerMode)

  err = ctx.ParseInput()
  require.NotNil(t, err)
  require.Equal(t, "invalid character 'h' looking for beginning of value", err.Error())
}
