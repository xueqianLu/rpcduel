// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
)

var contractERC721Cmd = &cobra.Command{
	Use:   "erc721",
	Short: "ERC-721 helpers (info, owner, tokenURI)",
}

var contractERC721InfoCmd = &cobra.Command{
	Use:   "info <token-address>",
	Short: "Print name, symbol, and (if Enumerable) totalSupply of an ERC-721 collection",
	Args:  cobra.ExactArgs(1),
	RunE:  runContractERC721Info,
}

var contractERC721OwnerCmd = &cobra.Command{
	Use:   "owner <token-address> <token-id>",
	Short: "Print the owner address of an ERC-721 token",
	Args:  cobra.ExactArgs(2),
	RunE:  runContractERC721Owner,
}

var contractERC721URICmd = &cobra.Command{
	Use:   "tokenURI <token-address> <token-id>",
	Short: "Print the metadata URI of an ERC-721 token",
	Args:  cobra.ExactArgs(2),
	RunE:  runContractERC721URI,
}

func init() {
	for _, c := range []*cobra.Command{contractERC721InfoCmd, contractERC721OwnerCmd, contractERC721URICmd} {
		addContractCommonFlags(c)
		contractERC721Cmd.AddCommand(c)
	}
}

func parseTokenID(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 0) // 0 base autodetects 0x prefix
	if !ok {
		return nil, fmt.Errorf("invalid token id %q", s)
	}
	return n, nil
}

func runContractERC721Info(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	info, err := c.ERC721Info(ctx, args[0], block)
	if err != nil {
		return err
	}
	if contractOutput == "json" {
		return printJSON(info)
	}
	fmt.Printf("Address:      %s\n", info.Address)
	fmt.Printf("Name:         %s\n", info.Name)
	fmt.Printf("Symbol:       %s\n", info.Symbol)
	if info.TotalSupply != nil {
		fmt.Printf("TotalSupply:  %s\n", info.TotalSupply.String())
	}
	for k, v := range info.Errors {
		// totalSupply revert is expected for non-Enumerable tokens; show it
		// as informational rather than alarming.
		fmt.Printf("  ! %s: %s\n", k, v)
	}
	return nil
}

func runContractERC721Owner(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	id, err := parseTokenID(args[1])
	if err != nil {
		return err
	}
	owner, err := c.ERC721OwnerOf(ctx, args[0], id, block)
	if err != nil {
		return err
	}
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"token": args[0], "token_id": id.String(), "owner": owner,
		})
	}
	fmt.Println(owner)
	return nil
}

func runContractERC721URI(_ *cobra.Command, args []string) error {
	c, ctx, cancel, err := newContractClient()
	if err != nil {
		return err
	}
	defer cancel()
	block, err := normalizeBlockTag(contractBlock)
	if err != nil {
		return err
	}
	id, err := parseTokenID(args[1])
	if err != nil {
		return err
	}
	uri, err := c.ERC721TokenURI(ctx, args[0], id, block)
	if err != nil {
		return err
	}
	if contractOutput == "json" {
		return printJSON(map[string]interface{}{
			"token": args[0], "token_id": id.String(), "tokenURI": uri,
		})
	}
	fmt.Println(uri)
	return nil
}
