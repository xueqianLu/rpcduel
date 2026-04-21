package benchgen

import (
	"math/rand"
	"testing"
)

func mkBF() *BenchFile {
	return &BenchFile{
		Version: "1",
		Scenarios: []Scenario{
			{Name: "balance", Weight: 0.2, Requests: []Request{
				{Method: "eth_getBalance", Params: []interface{}{"0xa", "latest"}},
				{Method: "eth_getBalance", Params: []interface{}{"0xb", "latest"}},
				{Method: "eth_getBalance", Params: []interface{}{"0xc", "latest"}},
			}},
			{Name: "block", Weight: 0.1, Requests: []Request{
				{Method: "eth_getBlockByNumber", Params: []interface{}{"0x1", false}},
				{Method: "eth_getBlockByNumber", Params: []interface{}{"0x2", false}},
			}},
			{Name: "logs", Weight: 0.1, Requests: []Request{
				{Method: "eth_getLogs", Params: []interface{}{map[string]interface{}{}}},
			}},
		},
	}
}

func TestFilterMethodsKeepsSubset(t *testing.T) {
	out := FilterMethods(mkBF(), []string{"eth_getBalance", "ETH_GETLOGS"})
	if len(out.Scenarios) != 2 {
		t.Fatalf("scenarios = %d, want 2 (balance,logs)", len(out.Scenarios))
	}
	for _, s := range out.Scenarios {
		for _, r := range s.Requests {
			if r.Method != "eth_getBalance" && r.Method != "eth_getLogs" {
				t.Errorf("unexpected method %s in filtered output", r.Method)
			}
		}
	}
}

func TestFilterMethodsEmptyReturnsOriginal(t *testing.T) {
	bf := mkBF()
	out := FilterMethods(bf, nil)
	if out != bf {
		t.Errorf("nil methods should return same pointer")
	}
}

func TestSampleKeepsAtLeastOnePerScenario(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	out := Sample(mkBF(), 0.001, rng)
	if len(out.Scenarios) != 3 {
		t.Fatalf("scenarios=%d, want 3", len(out.Scenarios))
	}
	for _, s := range out.Scenarios {
		if len(s.Requests) == 0 {
			t.Errorf("scenario %s lost all requests", s.Name)
		}
	}
}

func TestSamplePassThroughWhenFracInvalid(t *testing.T) {
	bf := mkBF()
	if Sample(bf, 0, nil) != bf {
		t.Errorf("frac=0 should pass through")
	}
	if Sample(bf, 1, nil) != bf {
		t.Errorf("frac=1 should pass through")
	}
}

func TestSampleDeterministicSeed(t *testing.T) {
	a := Sample(mkBF(), 0.5, rand.New(rand.NewSource(7)))
	b := Sample(mkBF(), 0.5, rand.New(rand.NewSource(7)))
	if len(a.Scenarios) != len(b.Scenarios) {
		t.Fatalf("non-deterministic scenario count")
	}
	for i := range a.Scenarios {
		if len(a.Scenarios[i].Requests) != len(b.Scenarios[i].Requests) {
			t.Errorf("scenario %d non-deterministic count", i)
		}
	}
}
