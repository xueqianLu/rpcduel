package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed assets/root_long.txt
var rootLong string

var providerMappings []string

var rootCmd = &cobra.Command{
	Use:           "rpcduel",
	Short:         "Stateless RPC audit and performance toolkit",
	Long:          strings.TrimSpace(rootLong),
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringArrayVar(
		&providerMappings,
		"provider",
		nil,
		"Provider alias mapping in the form alias=https://rpc.example",
	)

	rootCmd.AddCommand(datasetCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(benchCmd)
	rootCmd.AddCommand(callCmd)
}
