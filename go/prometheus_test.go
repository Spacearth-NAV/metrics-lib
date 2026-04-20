// Copyright 2025 Spacearth NAV S.r.l.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPrometheusAddObservation(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	p.AddObservation("requests_received", 1, Label{Key: "endpoint", Value: "/login"})
	p.AddObservation("requests_received", 2, Label{Key: "endpoint", Value: "/login"})

	got := testutil.ToFloat64(p.counters["requests_received"].WithLabelValues("/login"))
	if got != 3 {
		t.Fatalf("counter value = %v, want 3", got)
	}
}

func TestPrometheusMeasureTime(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	p.MeasureTime("processing_time", 120*time.Millisecond, Label{Key: "step", Value: "auth"})
	p.MeasureTime("processing_time", 80*time.Millisecond, Label{Key: "step", Value: "auth"})

	count := testutil.CollectAndCount(p.histograms["processing_time"])
	if count != 1 {
		t.Fatalf("histogram series count = %d, want 1", count)
	}

	exposed := mustGather(t, p)
	if !strings.Contains(exposed, `custom_processing_time_count{step="auth"} 2`) {
		t.Fatalf("expected histogram count 2 in exposition, got:\n%s", exposed)
	}
	if !strings.Contains(exposed, `custom_processing_time_sum{step="auth"} 0.2`) {
		t.Fatalf("expected histogram sum 0.2 in exposition, got:\n%s", exposed)
	}
}

func TestPrometheusGaugeIncrementDecrementSet(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	p.IncrementValue("active_connections", 3)
	p.DecrementValue("active_connections", 1)

	got := testutil.ToFloat64(p.gauges["active_connections"].WithLabelValues())
	if got != 2 {
		t.Fatalf("gauge after inc(3)/dec(1) = %v, want 2", got)
	}

	p.SetValue("queue_depth", 42)
	got = testutil.ToFloat64(p.gauges["queue_depth"].WithLabelValues())
	if got != 42 {
		t.Fatalf("gauge after set(42) = %v, want 42", got)
	}
}

func TestPrometheusFixedLabels(t *testing.T) {
	p := buildPrometheusServer("custom", []Label{{Key: "env", Value: "dev"}})

	p.AddObservation("requests_received", 1, Label{Key: "endpoint", Value: "/login"})

	exposed := mustGather(t, p)
	if !strings.Contains(exposed, `custom_requests_received{endpoint="/login",env="dev"} 1`) {
		t.Fatalf("expected fixed label 'env=dev' in exposition, got:\n%s", exposed)
	}
}

func TestPrometheusNamespacePrefix(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	p.SetValue("queue_depth", 7)

	exposed := mustGather(t, p)
	if !strings.Contains(exposed, "custom_queue_depth") {
		t.Fatalf("expected namespace prefix 'custom_' in exposition, got:\n%s", exposed)
	}
}

func TestPrometheusDedicatedRegistry(t *testing.T) {
	p := buildPrometheusServer("custom", nil)
	p.SetValue("queue_depth", 1)

	exposed := mustGather(t, p)
	for _, forbidden := range []string{"go_goroutines", "go_memstats_", "process_cpu_seconds_total"} {
		if strings.Contains(exposed, forbidden) {
			t.Fatalf("dedicated registry should not expose %q, got:\n%s", forbidden, exposed)
		}
	}
}

func TestPrometheusHTTPExposition(t *testing.T) {
	p := buildPrometheusServer("custom", nil)
	p.SetValue("queue_depth", 42, Label{Key: "queue", Value: "ingest"})

	ts := httptest.NewServer(promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{Registry: p.registry}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `custom_queue_depth{queue="ingest"} 42`) {
		t.Fatalf("exposition missing expected line, got:\n%s", string(body))
	}
}

func TestPrometheusMetricCachedByName(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	p.AddObservation("requests_received", 1, Label{Key: "endpoint", Value: "/a"})
	p.AddObservation("requests_received", 1, Label{Key: "endpoint", Value: "/b"})

	if len(p.counters) != 1 {
		t.Fatalf("expected 1 registered counter, got %d", len(p.counters))
	}

	if testutil.CollectAndCount(p.counters["requests_received"]) != 2 {
		t.Fatalf("expected two label sets on the shared counter")
	}
}

func TestPrometheusLabelSetMismatchDoesNotPanic(t *testing.T) {
	p := buildPrometheusServer("custom", []Label{{Key: "env", Value: "dev"}})

	// First call registers the metric with keys {endpoint, env}.
	p.AddObservation("requests_received", 1, Label{Key: "endpoint", Value: "/login"})

	// Second call uses a different key set ({method, env}). Must not panic,
	// the value must be dropped, and the original series must be untouched.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on label-set mismatch: %v", r)
		}
	}()
	p.AddObservation("requests_received", 42, Label{Key: "method", Value: "GET"})

	// The cached counter should still have exactly one series with the
	// original label set and its value should be 1 (not 43).
	if got := testutil.CollectAndCount(p.counters["requests_received"]); got != 1 {
		t.Fatalf("counter series count = %d, want 1", got)
	}
	if got := testutil.ToFloat64(p.counters["requests_received"].With(prometheus.Labels{"endpoint": "/login", "env": "dev"})); got != 1 {
		t.Fatalf("counter value = %v, want 1 (mismatch call must have been dropped)", got)
	}
}

func TestPrometheusLabelOrderIsInsensitive(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic when swapping label order: %v", r)
		}
	}()

	p.SetValue("queue_depth", 1, Label{Key: "queue", Value: "a"}, Label{Key: "region", Value: "eu"})
	// Same key set, swapped order — must be accepted, not treated as mismatch.
	p.SetValue("queue_depth", 7, Label{Key: "region", Value: "eu"}, Label{Key: "queue", Value: "a"})

	got := testutil.ToFloat64(p.gauges["queue_depth"].With(prometheus.Labels{"queue": "a", "region": "eu"}))
	if got != 7 {
		t.Fatalf("gauge value = %v, want 7", got)
	}
}

func TestPrometheusLabelSetMismatchAcrossAllMetricKinds(t *testing.T) {
	p := buildPrometheusServer("custom", nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	// counter
	p.AddObservation("c", 1, Label{Key: "a", Value: "1"})
	p.AddObservation("c", 1, Label{Key: "b", Value: "2"}) // dropped
	if got := testutil.CollectAndCount(p.counters["c"]); got != 1 {
		t.Fatalf("counter series = %d, want 1", got)
	}

	// histogram
	p.MeasureTime("h", time.Millisecond, Label{Key: "a", Value: "1"})
	p.MeasureTime("h", time.Millisecond, Label{Key: "b", Value: "2"}) // dropped
	if got := testutil.CollectAndCount(p.histograms["h"]); got != 1 {
		t.Fatalf("histogram series = %d, want 1", got)
	}

	// gauge (set/inc/dec share the same vec via getGauge)
	p.SetValue("g", 1, Label{Key: "a", Value: "1"})
	p.SetValue("g", 99, Label{Key: "b", Value: "2"}) // dropped
	p.IncrementValue("g", 5, Label{Key: "x", Value: "y"}) // dropped
	if got := testutil.CollectAndCount(p.gauges["g"]); got != 1 {
		t.Fatalf("gauge series = %d, want 1", got)
	}
	if got := testutil.ToFloat64(p.gauges["g"].With(prometheus.Labels{"a": "1"})); got != 1 {
		t.Fatalf("gauge value = %v, want 1", got)
	}
}

func TestPrometheusPortFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantVal int
	}{
		{name: "empty falls back to default", envVal: "", wantVal: defaultPrometheusPort},
		{name: "valid port is used", envVal: "9090", wantVal: 9090},
		{name: "non-numeric falls back to default", envVal: "abc", wantVal: defaultPrometheusPort},
		{name: "zero falls back to default", envVal: "0", wantVal: defaultPrometheusPort},
		{name: "negative falls back to default", envVal: "-1", wantVal: defaultPrometheusPort},
		{name: "above range falls back to default", envVal: "70000", wantVal: defaultPrometheusPort},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(prometheusPortEnvVar, tc.envVal)
			if got := prometheusPortFromEnv(); got != tc.wantVal {
				t.Fatalf("prometheusPortFromEnv() = %d, want %d", got, tc.wantVal)
			}
		})
	}
}

func mustGather(t *testing.T, p *prometheusServer) string {
	t.Helper()

	ts := httptest.NewServer(promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{Registry: p.registry}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("gather read: %v", err)
	}
	return string(body)
}
