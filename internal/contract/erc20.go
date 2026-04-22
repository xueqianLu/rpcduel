// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"context"
	"math/big"

	"github.com/xueqianLu/rpcduel/internal/abi"
)

// ERC20Info captures the metadata returned by ERC20Info.
type ERC20Info struct {
	Address     string   `json:"address"`
	Name        string   `json:"name,omitempty"`
	Symbol      string   `json:"symbol,omitempty"`
	Decimals    *uint8   `json:"decimals,omitempty"`
	TotalSupply *big.Int `json:"total_supply,omitempty"`
	// Errors records per-field errors (name/symbol/decimals/totalSupply).
	// Non-fatal: if a token is missing one method we still return what we have.
	Errors map[string]string `json:"errors,omitempty"`
}

// ERC20Info fetches name, symbol, decimals, and totalSupply in sequence
// (single endpoint, single block). Errors on individual fields are kept
// in Info.Errors so callers can render partial data.
func (c *Client) ERC20Info(ctx context.Context, address, block string) (*ERC20Info, error) {
	addr, err := abi.NormalizeAddress(address)
	if err != nil {
		return nil, err
	}
	info := &ERC20Info{Address: addr, Errors: map[string]string{}}

	// name() — bytes32 fallback for legacy tokens like MKR/SAI.
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

	if data, err := c.callSel(ctx, addr, abi.SelDecimals, block); err != nil {
		info.Errors["decimals"] = err.Error()
	} else if d, err := abi.DecodeUint8(data); err != nil {
		info.Errors["decimals"] = err.Error()
	} else {
		info.Decimals = &d
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

// ERC20BalanceOf returns the raw uint256 balance.
func (c *Client) ERC20BalanceOf(ctx context.Context, token, holder, block string) (*big.Int, error) {
	tokenAddr, err := abi.NormalizeAddress(token)
	if err != nil {
		return nil, err
	}
	holderAddr, err := abi.NormalizeAddress(holder)
	if err != nil {
		return nil, err
	}
	data, err := abi.EncodeCallData(abi.SelBalanceOf, abi.Address(holderAddr))
	if err != nil {
		return nil, err
	}
	out, err := c.CallContract(ctx, tokenAddr, data, block)
	if err != nil {
		return nil, err
	}
	return abi.DecodeUint256(out)
}

// ERC20Allowance returns the raw uint256 allowance(owner, spender).
func (c *Client) ERC20Allowance(ctx context.Context, token, owner, spender, block string) (*big.Int, error) {
	tokenAddr, err := abi.NormalizeAddress(token)
	if err != nil {
		return nil, err
	}
	ownerAddr, err := abi.NormalizeAddress(owner)
	if err != nil {
		return nil, err
	}
	spenderAddr, err := abi.NormalizeAddress(spender)
	if err != nil {
		return nil, err
	}
	data, err := abi.EncodeCallData(abi.SelAllowance, abi.Address(ownerAddr), abi.Address(spenderAddr))
	if err != nil {
		return nil, err
	}
	out, err := c.CallContract(ctx, tokenAddr, data, block)
	if err != nil {
		return nil, err
	}
	return abi.DecodeUint256(out)
}

// callSel is a tiny helper for parameterless selectors.
func (c *Client) callSel(ctx context.Context, addr string, sel [4]byte, block string) ([]byte, error) {
	data, err := abi.EncodeCallData(sel)
	if err != nil {
		return nil, err
	}
	return c.CallContract(ctx, addr, data, block)
}
