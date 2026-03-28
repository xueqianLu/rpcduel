package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrettyRPCResultAnnotatesQuantitiesButNotHashes(t *testing.T) {
	hash := "0x" + strings.Repeat("1", 64)
	raw := json.RawMessage(`{"number":"0x64","hash":"` + hash + `"}`)

	output, err := prettyRPCResult(raw)
	if err != nil {
		t.Fatalf("prettyRPCResult() error = %v", err)
	}

	if !strings.Contains(output, `"number": "0x64 (100)"`) {
		t.Fatalf("prettyRPCResult() did not annotate quantity:\n%s", output)
	}

	if !strings.Contains(output, `"hash": "`+hash+`"`) {
		t.Fatalf("prettyRPCResult() changed hash unexpectedly:\n%s", output)
	}

	if strings.Contains(output, `"hash": "`+hash+` (`) {
		t.Fatalf("prettyRPCResult() annotated hash unexpectedly:\n%s", output)
	}
}
