// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/abi"
	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// fakeRPC routes by (method, calldata-selector-prefix) to canned hex
// responses. Selector match uses the first 10 hex chars of the `data`
// field (0x + 4 bytes).
type fakeRPC struct {
	// key: method or "eth_call:<selector>"
	responses map[string]string
}

func (f *fakeRPC) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     interface{}     `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		key := req.Method
		if req.Method == "eth_call" {
			var params []json.RawMessage
			_ = json.Unmarshal(req.Params, &params)
			var tx struct {
				Data string `json:"data"`
			}
			_ = json.Unmarshal(params[0], &tx)
			if len(tx.Data) >= 10 {
				key = "eth_call:" + strings.ToLower(tx.Data[:10])
			}
		}
		result, ok := f.responses[key]
		if !ok {
			http.Error(w, "no canned response for "+key, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0", "id": req.ID, "result": result,
		})
	}
}

func newTestClient(t *testing.T, f *fakeRPC) *Client {
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return NewClient(rpc.NewClient(srv.URL, 5*time.Second))
}

// abiUint256 returns the 32-byte ABI encoding of n as a hex string.
func abiUint256(n *big.Int) string {
	enc, _ := abi.EncodeCallData([4]byte{0, 0, 0, 0}, abi.Uint256(n))
	return abi.ToHex(enc[4:])
}

// abiString encodes s as the dynamic-string return value (offset+len+data padded).
func abiString(s string) string {
	// offset = 0x20
	offset := strings.Repeat("0", 62) + "20"
	b := []byte(s)
	lenHex := abiUint256(big.NewInt(int64(len(b))))[2:] // strip 0x
	data := make([]byte, ((len(b)+31)/32)*32)
	copy(data, b)
	return "0x" + offset + lenHex + abi.ToHex(data)[2:]
}

func TestERC20InfoUSDCLike(t *testing.T) {
	f := &fakeRPC{responses: map[string]string{
		"eth_call:0x06fdde03": abiString("USD Coin"),                                                            // name
		"eth_call:0x95d89b41": abiString("USDC"),                                                                // symbol
		"eth_call:0x313ce567": "0x" + strings.Repeat("0", 63) + "6",                                             // decimals=6
		"eth_call:0x18160ddd": abiUint256(new(big.Int).Mul(big.NewInt(1_000_000_000), big.NewInt(1_000_000))), // 1e15
	}}
	c := newTestClient(t, f)
	info, err := c.ERC20Info(context.Background(), "0x1111111111111111111111111111111111111111", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "USD Coin" || info.Symbol != "USDC" || info.Decimals == nil || *info.Decimals != 6 {
		t.Errorf("info=%+v", info)
	}
	if info.TotalSupply == nil || info.TotalSupply.String() != "1000000000000000" {
		t.Errorf("totalSupply=%v", info.TotalSupply)
	}
	if info.Errors != nil {
		t.Errorf("unexpected errors: %v", info.Errors)
	}
}

func TestERC20BalanceOfAndAllowance(t *testing.T) {
	f := &fakeRPC{responses: map[string]string{
		"eth_call:0x70a08231": abiUint256(big.NewInt(123_456_789)),
		"eth_call:0xdd62ed3e": abiUint256(big.NewInt(42)),
	}}
	c := newTestClient(t, f)
	bal, err := c.ERC20BalanceOf(context.Background(),
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222", "")
	if err != nil || bal.Int64() != 123_456_789 {
		t.Errorf("bal=%v err=%v", bal, err)
	}
	al, err := c.ERC20Allowance(context.Background(),
		"0x1111111111111111111111111111111111111111",
		"0x2222222222222222222222222222222222222222",
		"0x3333333333333333333333333333333333333333", "")
	if err != nil || al.Int64() != 42 {
		t.Errorf("allowance=%v err=%v", al, err)
	}
}

func TestERC721OwnerOfTokenURI(t *testing.T) {
	owner := "0x" + strings.Repeat("0", 24) + "00000000000000000000000000000000000000ab"
	f := &fakeRPC{responses: map[string]string{
		"eth_call:0x6352211e": owner,
		"eth_call:0xc87b56dd": abiString("ipfs://Qm.../1"),
	}}
	c := newTestClient(t, f)
	got, err := c.ERC721OwnerOf(context.Background(),
		"0x1111111111111111111111111111111111111111", big.NewInt(1), "")
	if err != nil || got != "0x00000000000000000000000000000000000000ab" {
		t.Errorf("owner=%s err=%v", got, err)
	}
	uri, err := c.ERC721TokenURI(context.Background(),
		"0x1111111111111111111111111111111111111111", big.NewInt(1), "")
	if err != nil || uri != "ipfs://Qm.../1" {
		t.Errorf("uri=%s err=%v", uri, err)
	}
}

func TestGetCodeAndStorage(t *testing.T) {
	f := &fakeRPC{responses: map[string]string{
		"eth_getCode":       "0x6080604052",
		"eth_getStorageAt": "0x" + strings.Repeat("0", 63) + "1",
	}}
	c := newTestClient(t, f)
	code, err := c.GetCode(context.Background(), "0x1111111111111111111111111111111111111111", "")
	if err != nil || abi.ToHex(code) != "0x6080604052" {
		t.Errorf("code=%s err=%v", abi.ToHex(code), err)
	}
	st, err := c.GetStorageAt(context.Background(), "0x1111111111111111111111111111111111111111", "0x0", "")
	if err != nil || len(st) != 32 || st[31] != 1 {
		t.Errorf("storage=%v err=%v", st, err)
	}
}
