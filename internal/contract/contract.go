// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package contract provides high-level helpers to query well-known
// standard contracts (ERC-20, ERC-721) and a few generic on-chain
// objects (storage slot, contract code) over JSON-RPC. It builds on
// internal/abi for encoding/decoding and internal/rpc for transport.
package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xueqianLu/rpcduel/internal/abi"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// Client wraps a *rpc.Client with contract-aware helpers.
type Client struct {
	rpc *rpc.Client
}

// NewClient wraps c.
func NewClient(c *rpc.Client) *Client { return &Client{rpc: c} }

// CallContract performs an eth_call returning the raw result bytes.
// `block` may be a tag ("latest", "pending", "finalized", "safe") or a
// 0x-prefixed hex block number; an empty value defaults to "latest".
func (c *Client) CallContract(ctx context.Context, to string, data []byte, block string) ([]byte, error) {
	if block == "" {
		block = "latest"
	}
	tx := map[string]string{
		"to":   to,
		"data": abi.ToHex(data),
	}
	resp, _, err := c.rpc.Call(ctx, "eth_call", []interface{}{tx, block})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	var hex string
	if err := json.Unmarshal(resp.Result, &hex); err != nil {
		return nil, fmt.Errorf("eth_call result: %w (raw=%s)", err, string(resp.Result))
	}
	return abi.FromHex(hex)
}

// GetStorageAt returns the 32-byte word at slot of address at the given
// block. Returns the raw bytes (32 bytes long on success).
func (c *Client) GetStorageAt(ctx context.Context, address, slot, block string) ([]byte, error) {
	if block == "" {
		block = "latest"
	}
	if !strings.HasPrefix(slot, "0x") {
		slot = "0x" + slot
	}
	resp, _, err := c.rpc.Call(ctx, "eth_getStorageAt", []interface{}{address, slot, block})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	var hex string
	if err := json.Unmarshal(resp.Result, &hex); err != nil {
		return nil, fmt.Errorf("eth_getStorageAt result: %w (raw=%s)", err, string(resp.Result))
	}
	return abi.FromHex(hex)
}

// GetCode returns the deployed bytecode at address at the given block.
// An empty/0x return means the address is an EOA (or self-destructed).
func (c *Client) GetCode(ctx context.Context, address, block string) ([]byte, error) {
	if block == "" {
		block = "latest"
	}
	resp, _, err := c.rpc.Call(ctx, "eth_getCode", []interface{}{address, block})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	var hex string
	if err := json.Unmarshal(resp.Result, &hex); err != nil {
		return nil, fmt.Errorf("eth_getCode result: %w (raw=%s)", err, string(resp.Result))
	}
	return abi.FromHex(hex)
}
