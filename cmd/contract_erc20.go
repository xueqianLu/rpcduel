// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/contract"
)

var contractERC20Cmd = &cobra.Command{
	Use:   "erc20",
	Short: "ERC-20 helpers (info, balance, allowance)",
}

var contractERC20InfoCmd = &cobra.Command{
	Use:   "info <token-address>",
	Short: "Print name, symbol, decimals, and totalSupply of an ERC-20 token",
	Args:  cobra.ExactArgs(1),
	RunE:  runContractERC20Info,
}

var contractERC20BalanceCmd = &cobra.Command{
	Use:   "balance <token-address> <holder-address>",
	Short: "Print the ERC-20 balance of a holder, formatted with decimals",
	Args:  cobra.ExactArgs(2),
	RunE:  runContractERC20Balance,
}

var contractERC20AllowanceCmd = &cobra.Command{
	Use:   "allowance <token-address> <owner> <spender>",
	Short: "Print the ERC-20 allowance(owner, spender)",
	Args:  cobra.ExactArgs(3),
	RunE:  runContractERC20Allowance,
}

func init() {
	for _, c := range []*cobra.Command{contractERC20InfoCmd, contractERC20BalanceCmd, contractERC20AllowanceCmd} {
		addContractCommonFlags(c)
		contractERC20Cmd.AddCommand(c)
	}
}

func runContractERC20Info(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	info, err := c.ERC20Info(ctx, args[0], block)
	if err != nil {
		return err
	}
	if contractOutput == "json" {
		return printJSON(info)
	}
	fmt.Printf("Address:      %s\n", info.Address)
	fmt.Printf("Name:         %s\n", info.Name)
	fmt.Printf("Symbol:       %s\n", info.Symbol)
	if info.Decimals != nil {
		fmt.Printf("Decimals:     %d\n", *info.Decimals)
	}
	if info.TotalSupply != nil {
		dec := uint8(0)
		if info.Decimals != nil {
			dec = *info.Decimals
		}
		fmt.Printf("TotalSupply:  %s\n", contract.FormatBalanceLine(info.TotalSupply, dec, info.Symbol))
	}
	for k, v := range info.Errors {
		fmt.Printf("  ! %s: %s\n", k, v)
	}
	return nil
}

func runContractERC20Balance(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	bal, err := c.ERC20BalanceOf(ctx, args[0], args[1], block)
	if err != nil {
		return err
	}
	// Pull decimals + symbol so the human-readable output is meaningful.
	info, _ := c.ERC20Info(ctx, args[0], block)
	dec := uint8(0)
	sym := ""
	if info != nil {
		if info.Decimals != nil {
			dec = *info.Decimals
		}
		sym = info.Symbol
	}
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"token":     args[0],
			"holder":    args[1],
			"raw":       bal.String(),
			"decimals":  dec,
			"symbol":    sym,
			"formatted": contract.FormatUnits(bal, dec),
		})
	}
	fmt.Println(contract.FormatBalanceLine(bal, dec, sym))
	return nil
}

func runContractERC20Allowance(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	a, err := c.ERC20Allowance(ctx, args[0], args[1], args[2], block)
	if err != nil {
		return err
	}
	info, _ := c.ERC20Info(ctx, args[0], block)
	dec := uint8(0)
	sym := ""
	if info != nil {
		if info.Decimals != nil {
			dec = *info.Decimals
		}
		sym = info.Symbol
	}
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"token":     args[0],
			"owner":     args[1],
			"spender":   args[2],
			"raw":       a.String(),
			"decimals":  dec,
			"symbol":    sym,
			"formatted": contract.FormatUnits(a, dec),
		})
	}
	fmt.Println(contract.FormatBalanceLine(a, dec, sym))
	return nil
}
