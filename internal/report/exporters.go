package report

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/diff"
	"github.com/xueqianLu/rpcduel/internal/replay"
	"github.com/xueqianLu/rpcduel/internal/thresholds"
)

// HTML / Markdown / JUnit exporters.
//
// All exporters are self-contained: HTML embeds inline SVG bar charts
// (no external assets), Markdown uses GFM tables, JUnit emits a
// surefire-compatible <testsuites> document. Threshold breaches, when
// supplied, are surfaced in each format as either a banner (HTML/MD)
// or test failures (JUnit).

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// -----------------------------------------------------------------
// HTML exporters
// -----------------------------------------------------------------

const htmlPrelude = `<!doctype html>
<meta charset="utf-8">
<title>%s</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif;margin:2rem;color:#222}
h1{margin:0 0 .25rem 0}
h2{margin-top:2rem}
table{border-collapse:collapse;margin-top:.5rem}
th,td{border:1px solid #ddd;padding:.4rem .7rem;text-align:right;font-variant-numeric:tabular-nums}
th{background:#f5f5f5;text-align:center}
td.l{text-align:left}
.banner{padding:.6rem 1rem;border-radius:4px;margin:1rem 0;font-weight:600}
.pass{background:#e8f5e9;color:#1b5e20;border:1px solid #66bb6a}
.fail{background:#ffebee;color:#b71c1c;border:1px solid #ef5350}
.bar{background:#1976d2;height:14px;display:inline-block;vertical-align:middle}
.bar-bg{background:#eee;display:inline-block;width:160px;height:14px;vertical-align:middle;margin-right:6px}
small{color:#666}
</style>
<h1>%s</h1>
<small>generated %s</small>
`

func writeBanner(w io.Writer, breaches []thresholds.Breach, configured bool) {
	if !configured {
		return
	}
	if len(breaches) == 0 {
		fmt.Fprint(w, `<div class="banner pass">Thresholds: PASS</div>`)
		return
	}
	fmt.Fprint(w, `<div class="banner fail">Thresholds: FAIL`)
	fmt.Fprint(w, "<ul>")
	for _, b := range breaches {
		fmt.Fprintf(w, "<li>%s</li>", html.EscapeString(b.String()))
	}
	fmt.Fprint(w, "</ul></div>")
}

func bar(value, max float64) string {
	if max <= 0 {
		return ""
	}
	w := int(value / max * 160)
	if w < 1 && value > 0 {
		w = 1
	}
	if w > 160 {
		w = 160
	}
	return fmt.Sprintf(`<span class="bar-bg"><span class="bar" style="width:%dpx"></span></span>%.2f`, w, value)
}

// WriteBenchHTML writes a self-contained HTML bench report to w.
func WriteBenchHTML(w io.Writer, r BenchReport, breaches []thresholds.Breach, configured bool) error {
	fmt.Fprintf(w, htmlPrelude, "rpcduel bench report", "rpcduel bench report",
		html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	writeBanner(w, breaches, configured)
	maxP99 := 0.0
	maxQPS := 0.0
	for _, s := range r.Summaries {
		if v := ms(s.P99); v > maxP99 {
			maxP99 = v
		}
		if s.QPS > maxQPS {
			maxQPS = s.QPS
		}
	}
	fmt.Fprint(w, `<h2>Per-endpoint summary</h2><table>
<tr><th>Endpoint</th><th>Scenario</th><th>Total</th><th>Errors</th><th>Error %</th><th>QPS</th><th>Avg ms</th><th>P50</th><th>P95</th><th>P99</th><th>P999</th></tr>`)
	for _, s := range r.Summaries {
		fmt.Fprintf(w, `<tr><td class="l">%s</td><td class="l">%s</td><td>%d</td><td>%d</td><td>%.2f</td><td>%s</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%s</td><td>%.2f</td></tr>`,
			html.EscapeString(s.Endpoint), html.EscapeString(s.Scenario),
			s.Total, s.Errors, s.ErrorRate*100,
			bar(s.QPS, maxQPS),
			ms(s.AvgLatency), ms(s.P50), ms(s.P95),
			bar(ms(s.P99), maxP99),
			ms(s.P999),
		)
	}
	fmt.Fprint(w, "</table>")
	return nil
}

// WriteDuelHTML writes a duel HTML report.
func WriteDuelHTML(w io.Writer, r DuelReport, breaches []thresholds.Breach, configured bool) error {
	fmt.Fprintf(w, htmlPrelude, "rpcduel duel report", "rpcduel duel report",
		html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	writeBanner(w, breaches, configured)
	fmt.Fprintf(w, `<p><b>Method:</b> %s &middot; <b>Endpoints:</b> %s &middot; <b>Requests:</b> %d &middot; <b>Diffs:</b> %d (%.2f%%)</p>`,
		html.EscapeString(r.Method), html.EscapeString(strings.Join(r.Endpoints, " vs ")),
		r.Total, r.DiffCount, r.DiffRate*100)
	if len(r.Metrics) > 0 {
		writeMetricsTable(w, r.Metrics)
	}
	if len(r.Diffs) > 0 {
		writeDiffSamples(w, r.Diffs, 20)
	}
	return nil
}

func writeMetricsTable(w io.Writer, sums []bench.Summary) {
	fmt.Fprint(w, `<h2>Latency</h2><table>
<tr><th>Endpoint</th><th>Avg ms</th><th>P95</th><th>P99</th><th>QPS</th><th>Errors</th></tr>`)
	for _, s := range sums {
		fmt.Fprintf(w, `<tr><td class="l">%s</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%d</td></tr>`,
			html.EscapeString(s.Endpoint),
			ms(s.AvgLatency), ms(s.P95), ms(s.P99), s.QPS, s.Errors)
	}
	fmt.Fprint(w, "</table>")
}

func writeDiffSamples(w io.Writer, diffs []diff.Difference, limit int) {
	if limit > len(diffs) {
		limit = len(diffs)
	}
	fmt.Fprintf(w, "<h2>Sample diffs (%d / %d)</h2><table><tr><th>Path</th><th>Left</th><th>Right</th><th>Reason</th></tr>", limit, len(diffs))
	for _, d := range diffs[:limit] {
		fmt.Fprintf(w, `<tr><td class="l">%s</td><td class="l">%s</td><td class="l">%s</td><td class="l">%s</td></tr>`,
			html.EscapeString(d.Path), html.EscapeString(fmt.Sprintf("%v", d.Left)),
			html.EscapeString(fmt.Sprintf("%v", d.Right)), html.EscapeString(d.Reason))
	}
	fmt.Fprint(w, "</table>")
}

// WriteDiffHTML writes a diff HTML report.
func WriteDiffHTML(w io.Writer, r DiffReport, breaches []thresholds.Breach, configured bool) error {
	fmt.Fprintf(w, htmlPrelude, "rpcduel diff report", "rpcduel diff report",
		html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	writeBanner(w, breaches, configured)
	fmt.Fprintf(w, `<p><b>Method:</b> %s &middot; <b>Endpoints:</b> %s &middot; <b>Requests:</b> %d &middot; <b>Diffs:</b> %d</p>`,
		html.EscapeString(r.Method), html.EscapeString(strings.Join(r.Endpoints, " vs ")),
		r.Total, r.DiffCount)
	if len(r.Diffs) > 0 {
		writeDiffSamples(w, r.Diffs, 50)
	}
	return nil
}

// WriteReplayHTML writes a replay HTML report.
func WriteReplayHTML(w io.Writer, r *replay.Result, breaches []thresholds.Breach, configured bool) error {
	fmt.Fprintf(w, htmlPrelude, "rpcduel replay report", "rpcduel replay report",
		html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	writeBanner(w, breaches, configured)
	if r == nil {
		return nil
	}
	fmt.Fprintf(w, `<p><b>Accounts:</b> %d &middot; <b>Transactions:</b> %d &middot; <b>Blocks:</b> %d</p>
<p><b>Total requests:</b> %d &middot; <b>Successful:</b> %d (%.2f%%) &middot; <b>Diffs:</b> %d</p>`,
		r.AccountsTested, r.TransactionsTested, r.BlocksTested,
		r.TotalRequests, r.SuccessRequests, r.SuccessRate()*100, len(r.Diffs))
	sum := r.Summary()
	if len(sum) > 0 {
		cats := sortedCategoryKeys(sum)
		max := 0
		for _, c := range cats {
			if sum[c] > max {
				max = sum[c]
			}
		}
		fmt.Fprint(w, "<h2>Diffs by category</h2><table><tr><th>Category</th><th>Count</th><th>Distribution</th></tr>")
		for _, cat := range cats {
			count := sum[cat]
			width := 0
			if max > 0 {
				width = (count * 400) / max
			}
			fmt.Fprintf(w, `<tr><td class="l">%s</td><td>%d</td><td><span class="bar" style="display:inline-block;height:10px;width:%dpx;background:#d73a49"></span></td></tr>`,
				html.EscapeString(string(cat)), count, width)
		}
		fmt.Fprint(w, "</table>")
	}
	return nil
}

// sortedCategoryKeys returns the keys of m sorted lexicographically so
// that HTML/Markdown category tables are deterministic across runs.
func sortedCategoryKeys(m map[replay.DiffCategory]int) []replay.DiffCategory {
	out := make([]replay.DiffCategory, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// -----------------------------------------------------------------
// Markdown exporters
// -----------------------------------------------------------------

func mdBanner(b *strings.Builder, breaches []thresholds.Breach, configured bool) {
	if !configured {
		return
	}
	if len(breaches) == 0 {
		b.WriteString("\n> ✅ **Thresholds: PASS**\n")
		return
	}
	b.WriteString("\n> ❌ **Thresholds: FAIL**\n")
	for _, br := range breaches {
		fmt.Fprintf(b, "> - %s\n", br.String())
	}
}

// WriteBenchMarkdown writes a Markdown bench report.
func WriteBenchMarkdown(w io.Writer, r BenchReport, breaches []thresholds.Breach, configured bool) error {
	var b strings.Builder
	b.WriteString("# rpcduel bench report\n")
	fmt.Fprintf(&b, "_generated %s_\n", time.Now().UTC().Format(time.RFC3339))
	mdBanner(&b, breaches, configured)
	b.WriteString("\n| Endpoint | Scenario | Total | Errors | Error % | QPS | Avg ms | P50 | P95 | P99 | P999 |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, s := range r.Summaries {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f | %.2f |\n",
			s.Endpoint, s.Scenario, s.Total, s.Errors, s.ErrorRate*100, s.QPS,
			ms(s.AvgLatency), ms(s.P50), ms(s.P95), ms(s.P99), ms(s.P999))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// WriteDuelMarkdown writes a Markdown duel report.
func WriteDuelMarkdown(w io.Writer, r DuelReport, breaches []thresholds.Breach, configured bool) error {
	var b strings.Builder
	b.WriteString("# rpcduel duel report\n")
	fmt.Fprintf(&b, "_generated %s_\n", time.Now().UTC().Format(time.RFC3339))
	mdBanner(&b, breaches, configured)
	fmt.Fprintf(&b, "\n- Method: `%s`\n- Endpoints: %s\n- Requests: %d\n- Diffs: %d (%.2f%%)\n",
		r.Method, strings.Join(r.Endpoints, " vs "), r.Total, r.DiffCount, r.DiffRate*100)
	if len(r.Metrics) > 0 {
		b.WriteString("\n| Endpoint | Avg ms | P95 | P99 | QPS | Errors |\n|---|---:|---:|---:|---:|---:|\n")
		for _, s := range r.Metrics {
			fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f | %.2f | %d |\n",
				s.Endpoint, ms(s.AvgLatency), ms(s.P95), ms(s.P99), s.QPS, s.Errors)
		}
	}
	if len(r.Diffs) > 0 {
		writeDiffMarkdown(&b, r.Diffs, 20)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeDiffMarkdown(b *strings.Builder, diffs []diff.Difference, limit int) {
	if limit > len(diffs) {
		limit = len(diffs)
	}
	fmt.Fprintf(b, "\n## Sample diffs (%d / %d)\n\n| Path | Left | Right | Reason |\n|---|---|---|---|\n", limit, len(diffs))
	for _, d := range diffs[:limit] {
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n",
			mdEscape(d.Path), mdEscape(fmt.Sprintf("%v", d.Left)), mdEscape(fmt.Sprintf("%v", d.Right)), mdEscape(d.Reason))
	}
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

// WriteDiffMarkdown writes a Markdown diff report.
func WriteDiffMarkdown(w io.Writer, r DiffReport, breaches []thresholds.Breach, configured bool) error {
	var b strings.Builder
	b.WriteString("# rpcduel diff report\n")
	fmt.Fprintf(&b, "_generated %s_\n", time.Now().UTC().Format(time.RFC3339))
	mdBanner(&b, breaches, configured)
	fmt.Fprintf(&b, "\n- Method: `%s`\n- Endpoints: %s\n- Requests: %d\n- Diffs: %d\n",
		r.Method, strings.Join(r.Endpoints, " vs "), r.Total, r.DiffCount)
	if len(r.Diffs) > 0 {
		writeDiffMarkdown(&b, r.Diffs, 50)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// WriteReplayMarkdown writes a Markdown replay report.
func WriteReplayMarkdown(w io.Writer, r *replay.Result, breaches []thresholds.Breach, configured bool) error {
	var b strings.Builder
	b.WriteString("# rpcduel replay report\n")
	fmt.Fprintf(&b, "_generated %s_\n", time.Now().UTC().Format(time.RFC3339))
	mdBanner(&b, breaches, configured)
	if r == nil {
		_, err := io.WriteString(w, b.String())
		return err
	}
	fmt.Fprintf(&b, "\n- Accounts: %d\n- Transactions: %d\n- Blocks: %d\n- Total requests: %d\n- Successful: %d (%.2f%%)\n- Diffs: %d\n",
		r.AccountsTested, r.TransactionsTested, r.BlocksTested,
		r.TotalRequests, r.SuccessRequests, r.SuccessRate()*100, len(r.Diffs))
	sum := r.Summary()
	if len(sum) > 0 {
		cats := sortedCategoryKeys(sum)
		max := 0
		for _, c := range cats {
			if sum[c] > max {
				max = sum[c]
			}
		}
		b.WriteString("\n## Diffs by category\n\n| Category | Count | Share |\n|---|---:|---|\n")
		for _, cat := range cats {
			count := sum[cat]
			bar := ""
			if max > 0 {
				n := (count * 20) / max
				bar = strings.Repeat("█", n)
			}
			fmt.Fprintf(&b, "| %s | %d | %s |\n", cat, count, bar)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// -----------------------------------------------------------------
// JUnit exporters (surefire schema)
// -----------------------------------------------------------------

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Name    string       `xml:"name,attr"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	System    *junitSystem  `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

type junitSystem struct {
	Body string `xml:",chardata"`
}

func writeJUnit(w io.Writer, suites junitSuites) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(suites)
}

// breachesByEndpoint indexes a breach list by endpoint (empty key for
// aggregate breaches).
func breachesByEndpoint(breaches []thresholds.Breach) map[string][]thresholds.Breach {
	out := make(map[string][]thresholds.Breach)
	for _, b := range breaches {
		out[b.Endpoint] = append(out[b.Endpoint], b)
	}
	return out
}

// WriteBenchJUnit writes a JUnit XML report covering each endpoint as a
// suite. Each metric becomes a testcase that fails when the matching
// threshold breached.
func WriteBenchJUnit(w io.Writer, r BenchReport, breaches []thresholds.Breach) error {
	idx := breachesByEndpoint(breaches)
	suites := junitSuites{Name: "rpcduel.bench"}
	for _, s := range r.Summaries {
		suite := junitSuite{Name: s.Endpoint}
		for _, c := range []struct {
			name   string
			value  float64
			metric string
		}{
			{"avg_ms", ms(s.AvgLatency), "avg_ms"},
			{"p50_ms", ms(s.P50), "p50_ms"},
			{"p95_ms", ms(s.P95), "p95_ms"},
			{"p99_ms", ms(s.P99), "p99_ms"},
			{"p999_ms", ms(s.P999), "p999_ms"},
			{"error_rate", s.ErrorRate, "error_rate"},
			{"qps", s.QPS, "min_qps"},
		} {
			tc := junitCase{
				Name:      c.name,
				ClassName: s.Endpoint,
				System:    &junitSystem{Body: fmt.Sprintf("%g", c.value)},
			}
			for _, b := range idx[s.Endpoint] {
				if b.Metric == c.metric {
					tc.Failure = &junitFailure{
						Message: b.String(),
						Type:    "ThresholdBreach",
					}
					suite.Failures++
					break
				}
			}
			suite.Tests++
			suite.Cases = append(suite.Cases, tc)
		}
		suites.Suites = append(suites.Suites, suite)
	}
	// Aggregate breaches (no endpoint) — surface in a synthetic suite.
	if agg := idx[""]; len(agg) > 0 {
		suite := junitSuite{Name: "aggregate"}
		for _, b := range agg {
			suite.Tests++
			suite.Failures++
			suite.Cases = append(suite.Cases, junitCase{
				Name:      b.Metric,
				ClassName: "aggregate",
				Failure:   &junitFailure{Message: b.String(), Type: "ThresholdBreach"},
			})
		}
		suites.Suites = append(suites.Suites, suite)
	}
	return writeJUnit(w, suites)
}

// WriteDuelJUnit writes a JUnit XML report for the duel command.
func WriteDuelJUnit(w io.Writer, r DuelReport, breaches []thresholds.Breach) error {
	rep := BenchReport{Summaries: r.Metrics}
	idx := breachesByEndpoint(breaches)
	suites := junitSuites{Name: "rpcduel.duel"}
	// Endpoint-level suites mirror bench.
	for _, s := range rep.Summaries {
		suite := junitSuite{Name: s.Endpoint}
		add := func(name, metric string, value float64) {
			tc := junitCase{Name: name, ClassName: s.Endpoint,
				System: &junitSystem{Body: fmt.Sprintf("%g", value)}}
			for _, b := range idx[s.Endpoint] {
				if b.Metric == metric {
					tc.Failure = &junitFailure{Message: b.String(), Type: "ThresholdBreach"}
					suite.Failures++
					break
				}
			}
			suite.Tests++
			suite.Cases = append(suite.Cases, tc)
		}
		add("p95_ms", "p95_ms", ms(s.P95))
		add("p99_ms", "p99_ms", ms(s.P99))
		add("error_rate", "error_rate", s.ErrorRate)
		suites.Suites = append(suites.Suites, suite)
	}
	// Aggregate suite for diff_rate.
	agg := junitSuite{Name: "aggregate"}
	tc := junitCase{Name: "diff_rate", ClassName: "aggregate",
		System: &junitSystem{Body: fmt.Sprintf("%g", r.DiffRate)}}
	for _, b := range idx[""] {
		if b.Metric == "diff_rate" {
			tc.Failure = &junitFailure{Message: b.String(), Type: "ThresholdBreach"}
			agg.Failures++
		}
	}
	agg.Tests++
	agg.Cases = append(agg.Cases, tc)
	suites.Suites = append(suites.Suites, agg)
	return writeJUnit(w, suites)
}

// WriteDiffJUnit writes a JUnit XML report for the diff command.
func WriteDiffJUnit(w io.Writer, r DiffReport, breaches []thresholds.Breach) error {
	rate := 0.0
	if r.Total > 0 {
		rate = float64(r.DiffCount) / float64(r.Total)
	}
	suite := junitSuite{Name: "diff"}
	add := func(name string, value float64) {
		tc := junitCase{Name: name, ClassName: "diff",
			System: &junitSystem{Body: fmt.Sprintf("%g", value)}}
		for _, b := range breaches {
			if b.Metric == name {
				tc.Failure = &junitFailure{Message: b.String(), Type: "ThresholdBreach"}
				suite.Failures++
				break
			}
		}
		suite.Tests++
		suite.Cases = append(suite.Cases, tc)
	}
	add("diff_rate", rate)
	add("max_diffs", float64(r.DiffCount))
	return writeJUnit(w, junitSuites{Name: "rpcduel.diff", Suites: []junitSuite{suite}})
}

// WriteReplayJUnit writes a JUnit XML report for the replay command.
func WriteReplayJUnit(w io.Writer, r *replay.Result, breaches []thresholds.Breach) error {
	if r == nil {
		r = &replay.Result{}
	}
	mismatchRate := 0.0
	errorRate := 0.0
	if r.TotalRequests > 0 {
		mismatchRate = float64(len(r.Diffs)) / float64(r.TotalRequests)
		errorRate = 1 - r.SuccessRate()
	}
	suite := junitSuite{Name: "replay"}
	add := func(name string, value float64) {
		tc := junitCase{Name: name, ClassName: "replay",
			System: &junitSystem{Body: fmt.Sprintf("%g", value)}}
		for _, b := range breaches {
			if b.Metric == name {
				tc.Failure = &junitFailure{Message: b.String(), Type: "ThresholdBreach"}
				suite.Failures++
				break
			}
		}
		suite.Tests++
		suite.Cases = append(suite.Cases, tc)
	}
	add("mismatch_rate", mismatchRate)
	add("error_rate", errorRate)
	add("max_mismatch", float64(len(r.Diffs)))

	// One additional suite with a testcase per replay diff category so
	// CI runners surface the breakdown without anyone needing to read
	// the HTML/Markdown report.
	catSuite := junitSuite{Name: "replay_categories"}
	for _, cat := range sortedCategoryKeys(r.Summary()) {
		count := r.Summary()[cat]
		tc := junitCase{
			Name:      string(cat),
			ClassName: "replay.category",
			System:    &junitSystem{Body: fmt.Sprintf("%d", count)},
		}
		catSuite.Tests++
		catSuite.Cases = append(catSuite.Cases, tc)
	}
	return writeJUnit(w, junitSuites{Name: "rpcduel.replay", Suites: []junitSuite{suite, catSuite}})
}
