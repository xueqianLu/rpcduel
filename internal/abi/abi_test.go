// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"math/big"
	"strings"
	"testing"
)

func TestEncodeCallDataNoArgs(t *testing.T) {
	got, err := EncodeCallData(SelName)
	if err != nil {
		t.Fatal(err)
	}
	if ToHex(got) != "0x06fdde03" {
		t.Errorf("got %s", ToHex(got))
	}
}

func TestEncodeBalanceOf(t *testing.T) {
	got, err := EncodeCallData(SelBalanceOf, Address("0x0000000000000000000000000000000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	want := "0x70a082310000000000000000000000000000000000000000000000000000000000000001"
	if ToHex(got) != want {
		t.Errorf("got  %s\nwant %s", ToHex(got), want)
	}
}

func TestEncodeAllowance(t *testing.T) {
	got, err := EncodeCallData(SelAllowance,
		Address("0x000000000000000000000000000000000000000a"),
		Address("0x000000000000000000000000000000000000000b"),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "0xdd62ed3e" +
		"000000000000000000000000000000000000000000000000000000000000000a" +
		"000000000000000000000000000000000000000000000000000000000000000b"
	if ToHex(got) != want {
		t.Errorf("got  %s\nwant %s", ToHex(got), want)
	}
}

func TestEncodeUint256Big(t *testing.T) {
	n, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	got, err := EncodeCallData(SelOwnerOf, Uint256(n))
	if err != nil {
		t.Fatal(err)
	}
	dec, err := DecodeUint256(got[4:])
	if err != nil || dec.Cmp(n) != 0 {
		t.Errorf("decode roundtrip failed: %v", dec)
	}
}

func TestDecodeAddress(t *testing.T) {
	raw, _ := FromHex("0x0000000000000000000000001234567890abcdef1234567890abcdef12345678")
	got, err := DecodeAddress(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("got %s", got)
	}
}

func TestDecodeBoolUint8(t *testing.T) {
	one, _ := FromHex("0x" + strings.Repeat("0", 63) + "1")
	b, err := DecodeBool(one)
	if err != nil || !b {
		t.Errorf("expected true, got %v err=%v", b, err)
	}
	u, err := DecodeUint8(one)
	if err != nil || u != 1 {
		t.Errorf("expected 1, got %d err=%v", u, err)
	}
}

func TestDecodeString(t *testing.T) {
	hex := "0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000004" +
		"5553444300000000000000000000000000000000000000000000000000000000"
	raw, _ := FromHex(hex)
	got, err := DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "USDC" {
		t.Errorf("got %q", got)
	}
}

func TestDecodeStringOrBytes32Fallback(t *testing.T) {
	// MKR symbol fallback: bytes32("MKR") = 0x4d4b520000...0000
	raw, _ := FromHex("0x4d4b520000000000000000000000000000000000000000000000000000000000")
	got, err := DecodeStringOrBytes32(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "MKR" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeAddress(t *testing.T) {
	cases := map[string]string{
		"0x1234567890ABCDEF1234567890abcdef12345678": "0x1234567890abcdef1234567890abcdef12345678",
		"1234567890abcdef1234567890abcdef12345678":   "0x1234567890abcdef1234567890abcdef12345678",
	}
	for in, want := range cases {
		got, err := NormalizeAddress(in)
		if err != nil {
			t.Errorf("%s: err %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
	if _, err := NormalizeAddress("0xdead"); err == nil {
		t.Error("expected error for short address")
	}
}
