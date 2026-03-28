package diff

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"
)

type Options struct {
	IgnoreFields        map[string]struct{}
	QuantityFields      map[string]struct{}
	NullEqualsEmpty     bool
	NormalizeQuantities bool
}

func DefaultOptions() Options {
	return Options{
		IgnoreFields:        map[string]struct{}{},
		QuantityFields:      defaultQuantityFields(),
		NullEqualsEmpty:     true,
		NormalizeQuantities: true,
	}
}

func NewIgnoreSet(fields []string) map[string]struct{} {
	ignore := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		ignore[trimmed] = struct{}{}
	}
	return ignore
}

type Difference struct {
	Path   string `json:"path"`
	Left   any    `json:"left,omitempty"`
	Right  any    `json:"right,omitempty"`
	Reason string `json:"reason"`
}

func (d Difference) String() string {
	return fmt.Sprintf("%s: %v != %v (%s)", d.Path, d.Left, d.Right, d.Reason)
}

func DeepDiff(left, right map[string]any, ignoreFields []string) []Difference {
	opts := DefaultOptions()
	opts.IgnoreFields = NewIgnoreSet(ignoreFields)

	differences := make([]Difference, 0)
	compareValue("$", left, right, opts, &differences)
	return differences
}

func CompareJSON(left, right json.RawMessage, opts Options) ([]Difference, error) {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return nil, fmt.Errorf("decode left JSON: %w", err)
	}

	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return nil, fmt.Errorf("decode right JSON: %w", err)
	}

	differences := make([]Difference, 0)
	compareValue("$", leftValue, rightValue, opts, &differences)
	return differences, nil
}

func compareValue(path string, left, right any, opts Options, differences *[]Difference) {
	if valuesEqual(path, left, right, opts) {
		return
	}

	leftMap, leftIsMap := left.(map[string]any)
	rightMap, rightIsMap := right.(map[string]any)
	if leftIsMap && rightIsMap {
		compareMap(path, leftMap, rightMap, opts, differences)
		return
	}

	leftSlice, leftIsSlice := left.([]any)
	rightSlice, rightIsSlice := right.([]any)
	if leftIsSlice && rightIsSlice {
		compareSlice(path, leftSlice, rightSlice, opts, differences)
		return
	}

	*differences = append(*differences, Difference{
		Path:   path,
		Left:   left,
		Right:  right,
		Reason: "value mismatch",
	})
}

func compareMap(path string, left, right map[string]any, opts Options, differences *[]Difference) {
	keys := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	for key := range right {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		childPath := joinPath(path, key)
		if shouldIgnore(opts.IgnoreFields, childPath, key) {
			continue
		}

		leftValue, leftOK := left[key]
		rightValue, rightOK := right[key]

		switch {
		case !leftOK:
			*differences = append(*differences, Difference{
				Path:   childPath,
				Left:   nil,
				Right:  rightValue,
				Reason: "missing in left",
			})
		case !rightOK:
			*differences = append(*differences, Difference{
				Path:   childPath,
				Left:   leftValue,
				Right:  nil,
				Reason: "missing in right",
			})
		default:
			compareValue(childPath, leftValue, rightValue, opts, differences)
		}
	}
}

func compareSlice(path string, left, right []any, opts Options, differences *[]Difference) {
	if len(left) != len(right) {
		*differences = append(*differences, Difference{
			Path:   path,
			Left:   len(left),
			Right:  len(right),
			Reason: "array length mismatch",
		})
	}

	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}

	for index := 0; index < limit; index++ {
		compareValue(fmt.Sprintf("%s[%d]", path, index), left[index], right[index], opts, differences)
	}
}

func valuesEqual(path string, left, right any, opts Options) bool {
	if opts.NullEqualsEmpty && nullEqualsEmpty(left, right) {
		return true
	}

	if opts.NormalizeQuantities && shouldNormalizeQuantity(path, opts.QuantityFields) {
		leftNumeric, leftOK := normalizeNumeric(left)
		rightNumeric, rightOK := normalizeNumeric(right)
		if leftOK && rightOK {
			return leftNumeric.Cmp(rightNumeric) == 0
		}
	}

	return reflect.DeepEqual(left, right)
}

func shouldNormalizeQuantity(path string, quantityFields map[string]struct{}) bool {
	if path == "$" {
		return true
	}

	field := lastPathSegment(path)
	if field == "" {
		return false
	}

	_, ok := quantityFields[field]
	return ok
}

func lastPathSegment(path string) string {
	normalized := strings.TrimPrefix(stripIndexes(path), "$.")
	if normalized == "$" || normalized == "" {
		return ""
	}

	if index := strings.LastIndex(normalized, "."); index >= 0 {
		return normalized[index+1:]
	}
	return normalized
}

func nullEqualsEmpty(left, right any) bool {
	if left == nil && isEmptyCollection(right) {
		return true
	}
	if right == nil && isEmptyCollection(left) {
		return true
	}
	return false
}

func isEmptyCollection(value any) bool {
	switch typed := value.(type) {
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func normalizeNumeric(value any) (*big.Int, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, false
		}

		number := new(big.Int)
		if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
			if _, ok := number.SetString(trimmed[2:], 16); ok {
				return number, true
			}
			return nil, false
		}

		if _, ok := number.SetString(trimmed, 10); ok {
			return number, true
		}
	case float64:
		return big.NewInt(int64(typed)), true
	}

	return nil, false
}

func joinPath(path, key string) string {
	if path == "$" {
		return "$." + key
	}
	return path + "." + key
}

func shouldIgnore(ignore map[string]struct{}, path, key string) bool {
	if len(ignore) == 0 {
		return false
	}

	candidates := []string{
		key,
		path,
		strings.TrimPrefix(path, "$."),
		stripIndexes(strings.TrimPrefix(path, "$.")),
	}

	for _, candidate := range candidates {
		if _, ok := ignore[candidate]; ok {
			return true
		}
	}
	return false
}

func stripIndexes(path string) string {
	var builder strings.Builder
	builder.Grow(len(path))

	depth := 0
	for _, r := range path {
		switch r {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}

func defaultQuantityFields() map[string]struct{} {
	fields := []string{
		"balance",
		"baseFeePerGas",
		"blobGasPrice",
		"blobGasUsed",
		"blockNumber",
		"chainId",
		"cumulativeGasUsed",
		"difficulty",
		"effectiveGasPrice",
		"excessBlobGas",
		"gas",
		"gasLimit",
		"gasPrice",
		"gasUsed",
		"index",
		"l1Fee",
		"l1GasPrice",
		"l1GasUsed",
		"maxFeePerBlobGas",
		"maxFeePerGas",
		"maxPriorityFeePerGas",
		"nonce",
		"number",
		"size",
		"status",
		"timestamp",
		"totalDifficulty",
		"transactionIndex",
		"type",
		"value",
		"v",
		"yParity",
	}

	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		out[field] = struct{}{}
	}
	return out
}
