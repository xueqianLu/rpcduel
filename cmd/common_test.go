package cmd

import "testing"

func TestParseCLIParamLeavesBareNumbersAsStrings(t *testing.T) {
	value, err := parseCLIParam("12345")
	if err != nil {
		t.Fatalf("parseCLIParam() error = %v", err)
	}

	text, ok := value.(string)
	if !ok || text != "12345" {
		t.Fatalf("parseCLIParam() = %#v, want string 12345", value)
	}
}

func TestParseCLIParamDecodesBooleansAndJSON(t *testing.T) {
	value, err := parseCLIParam("false")
	if err != nil {
		t.Fatalf("parseCLIParam(false) error = %v", err)
	}
	boolean, ok := value.(bool)
	if !ok || boolean {
		t.Fatalf("parseCLIParam(false) = %#v, want bool false", value)
	}

	value, err = parseCLIParam(`["latest",false]`)
	if err != nil {
		t.Fatalf("parseCLIParam(array) error = %v", err)
	}
	array, ok := value.([]any)
	if !ok || len(array) != 2 {
		t.Fatalf("parseCLIParam(array) = %#v, want decoded array", value)
	}
}

func TestCoerceCallParamsConvertsDecimalBlockTags(t *testing.T) {
	params := coerceCallParams("eth_getBlockByNumber", []any{"12345", false})
	if got := params[0]; got != "0x3039" {
		t.Fatalf("coerceCallParams() first param = %#v, want 0x3039", got)
	}
}

func TestCoerceCallParamsLeavesNamedTagsUntouched(t *testing.T) {
	params := coerceCallParams("eth_getBalance", []any{"0xabc", "latest"})
	if got := params[1]; got != "latest" {
		t.Fatalf("coerceCallParams() second param = %#v, want latest", got)
	}
}
