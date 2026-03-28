package diff_test

import (
	"encoding/json"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/diff"
)

func TestDeepDiffHexNormalization(t *testing.T) {
	left := map[string]any{"number": "0x01"}
	right := map[string]any{"number": "0x1"}

	differences := diff.DeepDiff(left, right, nil)
	if len(differences) != 0 {
		t.Fatalf("DeepDiff() differences = %v, want none", differences)
	}
}

func TestCompareJSONDoesNotNormalizeHashFields(t *testing.T) {
	opts := diff.DefaultOptions()

	differences, err := diff.CompareJSON(
		json.RawMessage(`{"hash":"0x01"}`),
		json.RawMessage(`{"hash":"0x1"}`),
		opts,
	)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 1 {
		t.Fatalf("CompareJSON() differences = %v, want one hash mismatch", differences)
	}
}

func TestCompareJSONIgnoresNamedFields(t *testing.T) {
	opts := diff.DefaultOptions()
	opts.IgnoreFields = diff.NewIgnoreSet([]string{"withdrawalsRoot"})

	differences, err := diff.CompareJSON(
		json.RawMessage(`{"hash":"0xabc","withdrawalsRoot":"0x1"}`),
		json.RawMessage(`{"hash":"0xabc","withdrawalsRoot":"0x2"}`),
		opts,
	)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("CompareJSON() differences = %v, want none", differences)
	}
}

func TestCompareJSONTreatsNullAndEmptyCollectionAsEqual(t *testing.T) {
	opts := diff.DefaultOptions()

	differences, err := diff.CompareJSON(
		json.RawMessage(`{"logs":null,"uncles":[]}`),
		json.RawMessage(`{"logs":[],"uncles":null}`),
		opts,
	)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("CompareJSON() differences = %v, want none", differences)
	}
}

func TestCompareJSONReportsNestedDifferences(t *testing.T) {
	opts := diff.DefaultOptions()

	differences, err := diff.CompareJSON(
		json.RawMessage(`{"result":{"transactions":[{"hash":"0x1","gas":"0x5208"}]}}`),
		json.RawMessage(`{"result":{"transactions":[{"hash":"0x1","gas":"0x5209"}]}}`),
		opts,
	)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 1 {
		t.Fatalf("CompareJSON() difference count = %d, want 1", len(differences))
	}
	if differences[0].Path != "$.result.transactions[0].gas" {
		t.Fatalf("CompareJSON() path = %q, want gas path", differences[0].Path)
	}
}

func TestCompareJSONMatchesEthereumStylePayloads(t *testing.T) {
	opts := diff.DefaultOptions()

	left := json.RawMessage(`{
		"number":"0x10",
		"size":"0x01",
		"transactions":[{"hash":"0xabc","value":"0x0"}],
		"withdrawals":null
	}`)
	right := json.RawMessage(`{
		"number":"16",
		"size":"0x1",
		"transactions":[{"hash":"0xabc","value":"0x00"}],
		"withdrawals":[]
	}`)

	differences, err := diff.CompareJSON(left, right, opts)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("CompareJSON() differences = %v, want none", differences)
	}
}

func TestCompareJSONNormalizesRootQuantities(t *testing.T) {
	opts := diff.DefaultOptions()

	differences, err := diff.CompareJSON(
		json.RawMessage(`"0x01"`),
		json.RawMessage(`"1"`),
		opts,
	)
	if err != nil {
		t.Fatalf("CompareJSON() error = %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("CompareJSON() differences = %v, want none", differences)
	}
}
