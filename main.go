package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/spf13/cobra"
  "github.com/tlowerison/credential-1password/keystore"
  "github.com/tlowerison/credential-1password/op"
  "github.com/tlowerison/credential-1password/util"
)

func main() {
  ctx := util.NewContext(op.Op, keystore.NewKeystore(util.ServiceName), os.Stdin)

  var rootCmd *cobra.Command
  rootCmd = &cobra.Command{
    Use:   ctx.GetName(),
    Short: "credential helper for 1Password",
    Run: func(cmd *cobra.Command, _ []string) {
      fmt.Println(cmd.UsageString())
    },
  }

  getCmd := &cobra.Command{
    Use:    "get",
    Short:  "get credential by key",
    PreRun: util.PreRunWithInput(ctx),
    Run:   util.Run(ctx, Get),
  }

  storeCmd := &cobra.Command{
    Use:    "store",
    Short:  "store key=value pair",
    PreRun: util.PreRunWithInput(ctx),
    Run:    util.Run(ctx, Store),
  }

  eraseCmd := &cobra.Command{
    Use:    "erase",
    Short:  "erase credential by key",
    PreRun: util.PreRunWithInput(ctx),
    Run:    util.Run(ctx, Erase),
  }

  opCmd := &cobra.Command{
    Use:   "op",
    Short: "execute an op command using the current session token",
    Run:   util.RunWithArgs(ctx, Op),
  }

  configCmd := &cobra.Command{
    Use: "config",
    Short: fmt.Sprintf("get/set credential-1password configurations - {%s}", strings.Join(ConfigKeys, ",")),
    Args: cobra.RangeArgs(1, 2),
    Run:  util.RunWithArgs(ctx, Config),
  }

  rootCmd.PersistentFlags().StringVarP(&ctx.Flags.Mode, "mode", "m", "", "credential mode - predefined modes include {git,docker}; other modes can be used for basic file storage")

  opCmd.Flags().SetInterspersed(false)

  configCmd.Flags().BoolVarP(&ctx.Flags.Config_Vault_Create, "create", "c", false, "If setting the vault, and no vault exists with that name, will create a new vault.")

  cobra.EnableCommandSorting = false
  rootCmd.AddCommand(getCmd)
  rootCmd.AddCommand(storeCmd)
  rootCmd.AddCommand(eraseCmd)
  rootCmd.AddCommand(opCmd)
  rootCmd.AddCommand(configCmd)

  rootCmd.Execute()
}
