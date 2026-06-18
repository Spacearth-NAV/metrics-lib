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
	"net"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPrometheusServer(t *testing.T, fixedLabels ...Label) *prometheusServer {
	t.Helper()
	srv, err := newPrometheusServer("testns", 0, fixedLabels...)
	require.NoError(t, err)
	return srv.(*prometheusServer)
}

func TestAddObservation_incrementsCounter(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 3.0)

	got := testutil.ToFloat64(ps.getCounter("requests", nil).With(prometheus.Labels{}))
	assert.Equal(t, 3.0, got)
}

func TestAddObservation_accumulates(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 2.0)
	ps.AddObservation("requests", 5.0)

	got := testutil.ToFloat64(ps.getCounter("requests", nil).With(prometheus.Labels{}))
	assert.Equal(t, 7.0, got)
}

func TestMeasureTime_recordsHistogram(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.MeasureTime("latency", 300*time.Millisecond)
	ps.MeasureTime("latency", 700*time.Millisecond)

	expected := `
	# HELP testns_latency
	# TYPE testns_latency histogram
	testns_latency_bucket{le="0.005"} 0
	testns_latency_bucket{le="0.01"} 0
	testns_latency_bucket{le="0.025"} 0
	testns_latency_bucket{le="0.05"} 0
	testns_latency_bucket{le="0.1"} 0
	testns_latency_bucket{le="0.25"} 0
	testns_latency_bucket{le="0.5"} 1
	testns_latency_bucket{le="1"} 2
	testns_latency_bucket{le="2.5"} 2
	testns_latency_bucket{le="5"} 2
	testns_latency_bucket{le="10"} 2
	testns_latency_bucket{le="+Inf"} 2
	testns_latency_sum 1
	testns_latency_count 2
	`

	assert.NoError(t, testutil.GatherAndCompare(ps.registry, strings.NewReader(expected), "testns_latency"))
}

func TestGauge_incrementThenDecrement(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.IncrementValue("connections", 5.0)
	ps.DecrementValue("connections", 2.0)

	got := testutil.ToFloat64(ps.getGauge("connections", nil).With(prometheus.Labels{}))
	assert.Equal(t, 3.0, got)
}

func TestGauge_setValueOverwrites(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.IncrementValue("queue_depth", 10.0)
	ps.SetValue("queue_depth", 1.0)

	got := testutil.ToFloat64(ps.getGauge("queue_depth", nil).With(prometheus.Labels{}))
	assert.Equal(t, 1.0, got)
}

func TestFixedLabels_appearOnMetric(t *testing.T) {
	ps := newTestPrometheusServer(t, Label{"env", "prod"})
	ps.AddObservation("requests", 1.0)

	expected := `
	# HELP testns_requests
	# TYPE testns_requests counter
	testns_requests{env="prod"} 1
	`

	assert.NoError(t, testutil.GatherAndCompare(ps.registry, strings.NewReader(expected), "testns_requests"))
}

func TestLabelCollision_panics(t *testing.T) {
	ps := newTestPrometheusServer(t, Label{"env", "prod"})
	assert.Panics(t, func() {
		ps.AddObservation("requests", 1.0, Label{"env", "dev"})
	})
}

func TestSchemeLock_differentLabelKeysPanic(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 1.0, Label{"endpoint", "/a"})
	assert.Panics(t, func() {
		ps.AddObservation("requests", 1.0, Label{"method", "GET"})
	})
}

func TestPortConflict_returnsError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	_, err = newPrometheusServer("testns", port)
	assert.Error(t, err)
}
