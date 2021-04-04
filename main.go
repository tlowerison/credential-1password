package main

import (
  "fmt"
  "strings"

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

  configCmd := &cobra.Command{
    Use: "config",
    Short: fmt.Sprintf("get/set credential-1password configurations - {%s}", strings.Join(ConfigKeys, ",")),
    Args: cobra.RangeArgs(1, 2),
    Run:  util.RunWithArgs(ctx, Config),
  }

  rootCmd.PersistentFlags().StringVarP(&ctx.ModeFlag, "mode", "m", "", "credential mode - predefined modes include {git,docker}; other modes can be used for basic file storage")
  configCmd.Flags().BoolVarP(&ctx.ShouldCreateVault, "create", "c", false, "If setting the vault, and no vault exists with that name, will create a new vault.")

  rootCmd.AddCommand(configCmd)
  rootCmd.AddCommand(getCmd)
  rootCmd.AddCommand(storeCmd)
  rootCmd.AddCommand(eraseCmd)

  rootCmd.Execute()
}
