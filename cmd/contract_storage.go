// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/abi"
)

var contractStorageCmd = &cobra.Command{
	Use:   "storage <address> <slot>",
	Short: "Read a 32-byte storage slot of any address (eth_getStorageAt)",
	Long: `Read the 32-byte storage word at the given slot of address.

<slot> may be a 0x-hex value or a decimal number; it is normalized to 0x-hex
before sending. Use "rpcduel contract code" to inspect deployed bytecode.`,
	Args: cobra.ExactArgs(2),
	RunE: runContractStorage,
}

func init() { addContractCommonFlags(contractStorageCmd) }

func runContractStorage(_ *cobra.Command, args []string) error {
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
	raw, err := c.GetStorageAt(ctx, addr, args[1], block)
	if err != nil {
		return err
	}
	hex := abi.ToHex(raw)
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"address": addr, "slot": args[1], "block": block, "value": hex,
		})
	}
	fmt.Println(hex)
	return nil
}
