// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/contract"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// Shared flags for every `contract` subcommand.
var (
	contractRPC     string
	contractBlock   string
	contractOutput  string
	contractTimeout time.Duration
)

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Query well-known standard contracts (ERC-20, ERC-721) and on-chain objects",
	Long: `The contract command groups read-only convenience helpers for inspecting
well-known standard contracts and generic on-chain state.

Available subcommands:
  erc20    Query ERC-20 metadata, balance, allowance
  erc721   Query ERC-721 metadata, owner, tokenURI
  storage  Read a storage slot of any address (eth_getStorageAt)
  code     Fetch deployed bytecode of any address (eth_getCode)

For arbitrary read-only contract calls, use "rpcduel call" with eth_call.`,
}

func addContractCommonFlags(c *cobra.Command) {
	c.Flags().StringVar(&contractRPC, "rpc", "", "RPC endpoint URL")
	c.Flags().StringVar(&contractBlock, "block", "latest", "Block tag (latest|pending|finalized|safe) or 0x-hex / decimal block number")
	c.Flags().StringVar(&contractOutput, "output", "text", "Output format: text or json")
	c.Flags().DurationVar(&contractTimeout, "timeout", 15*time.Second, "Per-request timeout")
	_ = c.MarkFlagRequired("rpc")
}

// normalizeBlockTag turns user input ("latest", "12345", "0x1f") into
// what eth_call / eth_getStorageAt expect (tag string or 0x-hex number).
func normalizeBlockTag(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "latest":
		return "latest", nil
	case "pending", "earliest", "finalized", "safe":
		return strings.ToLower(strings.TrimSpace(s)), nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strings.ToLower(s), nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return "", fmt.Errorf("invalid --block %q", s)
	}
	return "0x" + n.Text(16), nil
}

func newContractClient() (*contract.Client, context.Context, context.CancelFunc, error) {
	if contractRPC == "" {
		return nil, nil, nil, fmt.Errorf("--rpc is required")
	}
	if err := validateOutputFormat(contractOutput); err != nil {
		return nil, nil, nil, err
	}
	c := contract.NewClient(rpc.NewClient(contractRPC, contractTimeout))
	ctx, cancel := context.WithTimeout(context.Background(), contractTimeout)
	return c, ctx, cancel, nil
}

// printJSON encodes v as indented JSON to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	contractCmd.AddCommand(contractERC20Cmd)
	contractCmd.AddCommand(contractERC721Cmd)
	contractCmd.AddCommand(contractStorageCmd)
	contractCmd.AddCommand(contractCodeCmd)
}
