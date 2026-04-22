// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"context"
	"math/big"

	"github.com/xueqianLu/rpcduel/internal/abi"
)

// ERC721Info captures the metadata returned by ERC721Info. totalSupply
// is only available for tokens that implement the ERC-721 Enumerable
// extension; if the call reverts, TotalSupply is left nil and the error
// is recorded in Errors["totalSupply"].
type ERC721Info struct {
	Address     string            `json:"address"`
	Name        string            `json:"name,omitempty"`
	Symbol      string            `json:"symbol,omitempty"`
	TotalSupply *big.Int          `json:"total_supply,omitempty"`
	Errors      map[string]string `json:"errors,omitempty"`
}

// ERC721Info fetches name, symbol, and (when implemented) totalSupply.
func (c *Client) ERC721Info(ctx context.Context, address, block string) (*ERC721Info, error) {
	addr, err := abi.NormalizeAddress(address)
	if err != nil {
		return nil, err
	}
	info := &ERC721Info{Address: addr, Errors: map[string]string{}}

	if data, err := c.callSel(ctx, addr, abi.SelName, block); err != nil {
		info.Errors["name"] = err.Error()
	} else if s, err := abi.DecodeStringOrBytes32(data); err != nil {
		info.Errors["name"] = err.Error()
	} else {
		info.Name = s
	}

	if data, err := c.callSel(ctx, addr, abi.SelSymbol, block); err != nil {
		info.Errors["symbol"] = err.Error()
	} else if s, err := abi.DecodeStringOrBytes32(data); err != nil {
		info.Errors["symbol"] = err.Error()
	} else {
		info.Symbol = s
	}

	if data, err := c.callSel(ctx, addr, abi.SelTotalSupply, block); err != nil {
		info.Errors["totalSupply"] = err.Error()
	} else if n, err := abi.DecodeUint256(data); err != nil {
		info.Errors["totalSupply"] = err.Error()
	} else {
		info.TotalSupply = n
	}

	if len(info.Errors) == 0 {
		info.Errors = nil
	}
	return info, nil
}

// ERC721OwnerOf returns the owner address of tokenId.
func (c *Client) ERC721OwnerOf(ctx context.Context, token string, tokenID *big.Int, block string) (string, error) {
	tokenAddr, err := abi.NormalizeAddress(token)
	if err != nil {
		return "", err
	}
	data, err := abi.EncodeCallData(abi.SelOwnerOf, abi.Uint256(tokenID))
	if err != nil {
		return "", err
	}
	out, err := c.CallContract(ctx, tokenAddr, data, block)
	if err != nil {
		return "", err
	}
	return abi.DecodeAddress(out)
}

// ERC721TokenURI returns the metadata URI of tokenId.
func (c *Client) ERC721TokenURI(ctx context.Context, token string, tokenID *big.Int, block string) (string, error) {
	tokenAddr, err := abi.NormalizeAddress(token)
	if err != nil {
		return "", err
	}
	data, err := abi.EncodeCallData(abi.SelTokenURI, abi.Uint256(tokenID))
	if err != nil {
		return "", err
	}
	out, err := c.CallContract(ctx, tokenAddr, data, block)
	if err != nil {
		return "", err
	}
	return abi.DecodeString(out)
}
