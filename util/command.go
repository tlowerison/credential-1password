package util

import (
  "fmt"
  "os"

  "github.com/spf13/cobra"
  "github.com/tlowerison/credential-1password/op"
)

type Runnable func(cmd *cobra.Command, args []string)

// PreRunWithInput wraps preRun with a session retry
func PreRunWithInput(ctx *Context) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, func(ctx *Context) error { return ctx.ReadInputs() })
  }
}

// Run
func Run(ctx *Context, fn func(ctx *Context) error) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, fn)
  }
}

// RunWithArgs
func RunWithArgs(ctx *Context, fn func(ctx *Context, args []string) error) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, func(ctx *Context) error { return fn(ctx, args) })
  }
}

// WithSessionRetry
func WithSessionRetry(ctx *Context, cmd *cobra.Command, args []string, fn func(ctx *Context) error) {
  ctx.SetCmd(cmd)
  err := fn(ctx)
  if op.ShouldClearSessionAndRetry(err) {
    ctx.Signin()
    err = fn(ctx)
  }
  HandleErr(err)
}

// HandleErr
func HandleErr(err error) {
  if err != nil {
    fmt.Fprintln(os.Stderr, err.Error())
    os.Exit(1)
  }
}
