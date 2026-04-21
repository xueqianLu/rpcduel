// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package diff provides deep JSON comparison for RPC responses.
package diff

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

// Options configures the diff engine behavior.
type Options struct {
	// IgnoreFields is a set of JSON key names to skip during comparison.
	IgnoreFields map[string]bool
	// IgnoreOrder when true treats arrays as sets (order-insensitive).
	IgnoreOrder bool
	// NormalizeHex when true treats "0x1a" and "26" as equal.
	NormalizeHex bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		IgnoreFields: map[string]bool{},
		IgnoreOrder:  false,
		NormalizeHex: true,
	}
}

// Difference records a single detected difference between two JSON values.
type Difference struct {
	Path   string
	Left   interface{}
	Right  interface{}
	Reason string
}

func (d Difference) String() string {
	return fmt.Sprintf("[%s] %v vs %v (%s)", d.Path, d.Left, d.Right, d.Reason)
}

// Compare deeply compares two raw JSON messages and returns any differences.
func Compare(left, right json.RawMessage, opts Options) ([]Difference, error) {
	var l, r interface{}
	if err := json.Unmarshal(left, &l); err != nil {
		return nil, fmt.Errorf("unmarshal left: %w", err)
	}
	if err := json.Unmarshal(right, &r); err != nil {
		return nil, fmt.Errorf("unmarshal right: %w", err)
	}
	var diffs []Difference
	compareValues("$", l, r, opts, &diffs)
	return diffs, nil
}

func compareValues(path string, l, r interface{}, opts Options, diffs *[]Difference) {
	if opts.NormalizeHex {
		lNorm, lOk := normalizeNumeric(l)
		rNorm, rOk := normalizeNumeric(r)
		if lOk && rOk {
			if lNorm.Cmp(rNorm) != 0 {
				*diffs = append(*diffs, Difference{
					Path:   path,
					Left:   l,
					Right:  r,
					Reason: "value mismatch",
				})
			}
			return
		}
	}

	lMap, lIsMap := toMap(l)
	rMap, rIsMap := toMap(r)
	if lIsMap && rIsMap {
		compareMaps(path, lMap, rMap, opts, diffs)
		return
	}

	lArr, lIsArr := toSlice(l)
	rArr, rIsArr := toSlice(r)
	if lIsArr && rIsArr {
		compareArrays(path, lArr, rArr, opts, diffs)
		return
	}

	if !reflect.DeepEqual(l, r) {
		*diffs = append(*diffs, Difference{
			Path:   path,
			Left:   l,
			Right:  r,
			Reason: "value mismatch",
		})
	}
}

func compareMaps(path string, l, r map[string]interface{}, opts Options, diffs *[]Difference) {
	keys := mergeKeys(l, r)
	for _, k := range keys {
		if opts.IgnoreFields[k] {
			continue
		}
		childPath := path + "." + k
		lv, lOk := l[k]
		rv, rOk := r[k]
		if !lOk {
			*diffs = append(*diffs, Difference{Path: childPath, Left: nil, Right: rv, Reason: "missing in left"})
			continue
		}
		if !rOk {
			*diffs = append(*diffs, Difference{Path: childPath, Left: lv, Right: nil, Reason: "missing in right"})
			continue
		}
		compareValues(childPath, lv, rv, opts, diffs)
	}
}

func compareArrays(path string, l, r []interface{}, opts Options, diffs *[]Difference) {
	if opts.IgnoreOrder {
		lStr := marshalAll(l)
		rStr := marshalAll(r)
		sort.Strings(lStr)
		sort.Strings(rStr)
		if !reflect.DeepEqual(lStr, rStr) {
			*diffs = append(*diffs, Difference{Path: path, Left: l, Right: r, Reason: "array elements differ (order-insensitive)"})
		}
		return
	}
	if len(l) != len(r) {
		*diffs = append(*diffs, Difference{
			Path:   path,
			Left:   fmt.Sprintf("len=%d", len(l)),
			Right:  fmt.Sprintf("len=%d", len(r)),
			Reason: "array length mismatch",
		})
		// still compare up to min length
	}
	min := len(l)
	if len(r) < min {
		min = len(r)
	}
	for i := 0; i < min; i++ {
		compareValues(fmt.Sprintf("%s[%d]", path, i), l[i], r[i], opts, diffs)
	}
}

// normalizeNumeric attempts to parse a value as a big integer (hex or decimal).
func normalizeNumeric(v interface{}) (*big.Int, bool) {
	s, ok := v.(string)
	if !ok {
		// Try float64 (JSON numbers)
		f, ok2 := v.(float64)
		if !ok2 {
			return nil, false
		}
		return big.NewInt(int64(f)), true
	}
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n := new(big.Int)
		_, success := n.SetString(s[2:], 16)
		return n, success
	}
	n := new(big.Int)
	_, success := n.SetString(s, 10)
	return n, success
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func toSlice(v interface{}) ([]interface{}, bool) {
	s, ok := v.([]interface{})
	return s, ok
}

func mergeKeys(a, b map[string]interface{}) []string {
	seen := make(map[string]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func marshalAll(items []interface{}) []string {
	out := make([]string, len(items))
	for i, item := range items {
		b, _ := json.Marshal(item)
		out[i] = string(b)
	}
	return out
}
