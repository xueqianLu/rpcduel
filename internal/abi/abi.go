// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package abi contains a minimal, hand-rolled ABI encoder/decoder
// covering only the types rpcduel currently needs to query standard
// contracts (ERC-20, ERC-721) — namely address, uintN, bool, bytes4,
// bytes32, and dynamic string. We deliberately avoid pulling in
// go-ethereum to keep the binary slim.
//
// If/when rpcduel grows generic ABI ingestion (arbitrary user-supplied
// ABIs, tuple types, dynamic arrays, etc.), swap this package for
// github.com/ethereum/go-ethereum/accounts/abi.
package abi

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"
)

// FromHex strips an optional 0x prefix and decodes the hex string.
// Empty / "0x" inputs decode to a zero-length slice.
func FromHex(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return nil, nil
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

// ToHex returns the 0x-prefixed lowercase hex of b.
func ToHex(b []byte) string { return "0x" + hex.EncodeToString(b) }

// NormalizeAddress validates s as a 20-byte hex address (with or without
// 0x prefix) and returns it as a 0x-prefixed lowercase string.
func NormalizeAddress(s string) (string, error) {
	raw, err := FromHex(s)
	if err != nil {
		return "", fmt.Errorf("address %q: %w", s, err)
	}
	if len(raw) != 20 {
		return "", fmt.Errorf("address %q: want 20 bytes, got %d", s, len(raw))
	}
	return ToHex(raw), nil
}

func padLeft32(b []byte) []byte {
	if len(b) >= 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// EncodeCallData builds calldata = selector || args. Supports the static
// argument types rpcduel currently needs (address, uint256, bytes4) — all
// 32-byte words, no offsets/dynamic types.
func EncodeCallData(selector [4]byte, args ...Arg) ([]byte, error) {
	out := make([]byte, 0, 4+32*len(args))
	out = append(out, selector[:]...)
	for i, a := range args {
		w, err := a.word()
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		out = append(out, w...)
	}
	return out, nil
}

// Arg is a typed ABI input. Construct via Address/Uint256/Bytes4.
type Arg struct {
	kind string
	addr string
	num  *big.Int
	b4   [4]byte
}

func Address(s string) Arg   { return Arg{kind: "address", addr: s} }
func Uint256(n *big.Int) Arg { return Arg{kind: "uint256", num: new(big.Int).Set(n)} }
func Bytes4(b [4]byte) Arg   { return Arg{kind: "bytes4", b4: b} }

func (a Arg) word() ([]byte, error) {
	switch a.kind {
	case "address":
		raw, err := FromHex(a.addr)
		if err != nil || len(raw) != 20 {
			return nil, fmt.Errorf("invalid address %q", a.addr)
		}
		return padLeft32(raw), nil
	case "uint256":
		if a.num == nil || a.num.Sign() < 0 {
			return nil, errors.New("uint256 must be non-negative")
		}
		buf := a.num.Bytes()
		if len(buf) > 32 {
			return nil, errors.New("uint256 overflow")
		}
		return padLeft32(buf), nil
	case "bytes4":
		out := make([]byte, 32)
		copy(out, a.b4[:]) // bytesN is right-padded with zeros
		return out, nil
	}
	return nil, fmt.Errorf("unsupported arg kind %q", a.kind)
}

// DecodeUint256 reads the first 32-byte word of data as a uint256.
func DecodeUint256(data []byte) (*big.Int, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("uint256: want >=32 bytes, got %d", len(data))
	}
	return new(big.Int).SetBytes(data[:32]), nil
}

// DecodeUint8 reads the first 32-byte word as uint8 (must fit in 1 byte).
func DecodeUint8(data []byte) (uint8, error) {
	n, err := DecodeUint256(data)
	if err != nil {
		return 0, err
	}
	if n.BitLen() > 8 {
		return 0, fmt.Errorf("uint8 overflow: %s", n.String())
	}
	return uint8(n.Uint64()), nil
}

// DecodeBool reads the first 32-byte word as a 0/1 bool.
func DecodeBool(data []byte) (bool, error) {
	n, err := DecodeUint256(data)
	if err != nil {
		return false, err
	}
	return n.Sign() != 0, nil
}

// DecodeAddress reads the rightmost 20 bytes of the first 32-byte word.
func DecodeAddress(data []byte) (string, error) {
	if len(data) < 32 {
		return "", fmt.Errorf("address: want >=32 bytes, got %d", len(data))
	}
	return ToHex(data[12:32]), nil
}

// DecodeString decodes an ABI-encoded dynamic string return value.
//
// Layout for a single-return-value call:
//
//	[0:32]   offset of the data (always 0x20 in single-return-value calls)
//	[off:]   uint256 length, then `length` bytes padded to 32-byte boundary.
func DecodeString(data []byte) (string, error) {
	if len(data) < 64 {
		return "", fmt.Errorf("string: want >=64 bytes, got %d", len(data))
	}
	off, err := DecodeUint256(data[:32])
	if err != nil {
		return "", err
	}
	o := int(off.Int64())
	if o < 32 || o+32 > len(data) {
		return "", fmt.Errorf("string: bad offset %d (data len %d)", o, len(data))
	}
	lenBig, err := DecodeUint256(data[o : o+32])
	if err != nil {
		return "", err
	}
	l := int(lenBig.Int64())
	start := o + 32
	if l < 0 || start+l > len(data) {
		return "", fmt.Errorf("string: bad length %d (data len %d, start %d)", l, len(data), start)
	}
	return string(data[start : start+l]), nil
}

// DecodeStringOrBytes32 first tries to decode `data` as a dynamic string.
// If that fails and `data` is exactly 32 bytes (the bytes32 fallback used
// by legacy tokens like MKR/SAI), it strips trailing zero bytes and
// returns the printable prefix.
func DecodeStringOrBytes32(data []byte) (string, error) {
	if s, err := DecodeString(data); err == nil {
		return s, nil
	}
	if len(data) == 32 {
		end := 32
		for end > 0 && data[end-1] == 0 {
			end--
		}
		s := string(data[:end])
		if !utf8.ValidString(s) {
			return "", fmt.Errorf("bytes32 string: not valid UTF-8 (raw=%s)", ToHex(data))
		}
		return s, nil
	}
	return "", fmt.Errorf("string/bytes32: unrecognized layout (len=%d)", len(data))
}

// Selectors used by rpcduel's standard contract helpers. Each is the
// first 4 bytes of keccak256(canonical signature). Verified against the
// 4byte directory.
var (
	SelName              = [4]byte{0x06, 0xfd, 0xde, 0x03} // name()
	SelSymbol            = [4]byte{0x95, 0xd8, 0x9b, 0x41} // symbol()
	SelDecimals          = [4]byte{0x31, 0x3c, 0xe5, 0x67} // decimals()
	SelTotalSupply       = [4]byte{0x18, 0x16, 0x0d, 0xdd} // totalSupply()
	SelBalanceOf         = [4]byte{0x70, 0xa0, 0x82, 0x31} // balanceOf(address)
	SelAllowance         = [4]byte{0xdd, 0x62, 0xed, 0x3e} // allowance(address,address)
	SelOwnerOf           = [4]byte{0x63, 0x52, 0x21, 0x1e} // ownerOf(uint256)
	SelTokenURI          = [4]byte{0xc8, 0x7b, 0x56, 0xdd} // tokenURI(uint256)
	SelSupportsInterface = [4]byte{0x01, 0xff, 0xc9, 0xa7} // supportsInterface(bytes4)
)
