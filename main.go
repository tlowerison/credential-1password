package main

import (
  "fmt"

  "github.com/spf13/cobra"
  "github.com/tlowerison/credential-1password/util"
)

func main() {
  ctx, err := util.Register("credential-1password")
  util.HandleErr(err)

  var rootCmd *cobra.Command
  rootCmd = &cobra.Command{
    Use:   ctx.GetName(),
    Short: "credential helper for 1Password",
    Run: func(cmd *cobra.Command, _ []string) {
      fmt.Println(cmd.UsageString())
    },
  }

  vaultCmd := &cobra.Command{
    Use:   "vault",
    Short: "get/set the vault that credential uses",
    Args:  cobra.RangeArgs(0, 1),
    Run:   util.RunWithArgs(ctx, Vault),
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

  rootCmd.PersistentFlags().StringVarP(&ctx.ModeFlag, "mode", "m", "", "credential mode - predefined modes include {git,docker}; other modes can be used for basic file storage")
  vaultCmd.Flags().BoolVarP(&ctx.ShouldCreateVault, "create", "c", false, "If setting the vault name and no vault exists with that name, will create a new vault.")

  rootCmd.AddCommand(vaultCmd)
  rootCmd.AddCommand(getCmd)
  rootCmd.AddCommand(storeCmd)
  rootCmd.AddCommand(eraseCmd)

  rootCmd.Execute()
}
