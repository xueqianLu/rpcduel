package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObserve_ExposesMetrics(t *testing.T) {
	Observe("https://node-a.example", "eth_blockNumber", 12*time.Millisecond, false)
	Observe("https://node-a.example", "eth_blockNumber", 50*time.Millisecond, true)
	ObserveDiff("https://node-a.example", "https://node-b.example", 3)

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	for _, want := range []string{
		`rpcduel_requests_total{endpoint="https://node-a.example",scenario="eth_blockNumber",status="ok"} 1`,
		`rpcduel_requests_total{endpoint="https://node-a.example",scenario="eth_blockNumber",status="error"} 1`,
		`rpcduel_diffs_total{endpoint_a="https://node-a.example",endpoint_b="https://node-b.example"} 3`,
		`rpcduel_request_duration_seconds_bucket`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("metrics output missing %q\n--- output ---\n%s", want, got)
		}
	}
}
