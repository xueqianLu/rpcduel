// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/abi"
)

var (
	contractCodeShowFull bool
)

var contractCodeCmd = &cobra.Command{
	Use:   "code <address>",
	Short: "Fetch deployed bytecode of any address (eth_getCode)",
	Long: `Fetch the deployed bytecode at address.

By default only the size and the first 64 bytes of code are printed (so
you can quickly tell EOA vs contract); pass --full to print the full
bytecode hex.`,
	Args: cobra.ExactArgs(1),
	RunE: runContractCode,
}

func init() {
	addContractCommonFlags(contractCodeCmd)
	contractCodeCmd.Flags().BoolVar(&contractCodeShowFull, "full", false, "Print the full bytecode hex (default: only size + first 64 bytes)")
}

func runContractCode(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	addr, err := abi.NormalizeAddress(args[0])
	if err != nil {
		return err
	}
	code, err := c.GetCode(ctx, addr, block)
	if err != nil {
		return err
	}
	isEOA := len(code) == 0
	preview := code
	if !contractCodeShowFull && len(preview) > 64 {
		preview = preview[:64]
	}
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"address": addr,
			"block":   block,
			"size":    len(code),
			"is_eoa":  isEOA,
			"code":    abi.ToHex(code),
			"preview": abi.ToHex(preview),
		})
	}
	if isEOA {
		fmt.Printf("Address:  %s\n", addr)
		fmt.Println("Code:     (none — externally owned account or self-destructed)")
		return nil
	}
	fmt.Printf("Address:  %s\n", addr)
	fmt.Printf("Size:     %d bytes\n", len(code))
	if contractCodeShowFull {
		fmt.Printf("Code:     %s\n", abi.ToHex(code))
	} else {
		fmt.Printf("Preview:  %s%s\n", abi.ToHex(preview),
			func() string {
				if len(code) > 64 {
					return "..."
				}
				return ""
			}())
		fmt.Println("(use --full to print the full bytecode)")
	}
	return nil
}
