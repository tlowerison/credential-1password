package util

import (
  "fmt"
  "os"

  "github.com/spf13/cobra"
  "github.com/tlowerison/credential-1password/op"
)

type Runnable func(cmd *cobra.Command, args []string)

// PreRunWithInput wraps ctx.ParseInput with a session retry.
func PreRunWithInput(ctx *Context) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, func(ctx *Context) error { return ctx.ParseInput() })
  }
}

// Run wraps fn with a session retry.
func Run(ctx *Context, fn func(ctx *Context) error) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, fn)
  }
}

// RunWithArgs fn with a session retry and passes args to fn.
func RunWithArgs(ctx *Context, fn func(ctx *Context, args []string) error) Runnable {
  return func(cmd *cobra.Command, args []string) {
    WithSessionRetry(ctx, cmd, args, func(ctx *Context) error { return fn(ctx, args) })
  }
}

// WithSessionRetry runs fn and if it receives an error which the op utils
// recognizes as an indication that a session token is missing or out of date,
// then it will request a new signin to generate a new session token and then
// run fn again with the new session token.
func WithSessionRetry(ctx *Context, cmd *cobra.Command, args []string, fn func(ctx *Context) error) {
  ctx.SetCmd(cmd)
  err := fn(ctx)
  if op.ShouldClearSessionAndRetry(err) {
    ctx.Signin()
    err = fn(ctx)
  }
  HandleErr(err)
}

// HandleErr does if nothing if the provided error is nil, otherwise
// it prints the provided err's message to stderr and exits.
func HandleErr(err error) {
  if err != nil {
    fmt.Fprintln(os.Stderr, err.Error())
    os.Exit(1)
  }
}
