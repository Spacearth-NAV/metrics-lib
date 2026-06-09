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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newTestPrometheusServer(t *testing.T, fixedLabels ...Label) *prometheusServer {
	t.Helper()
	srv, err := newPrometheusServer("testns", 0, fixedLabels...)
	if err != nil {
		t.Fatalf("newPrometheusServer: %v", err)
	}
	return srv.(*prometheusServer)
}

func TestAddObservation_incrementsCounter(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 3.0)

	got := testutil.ToFloat64(ps.getCounter("requests", nil).With(prometheus.Labels{}))
	if got != 3.0 {
		t.Errorf("want 3.0, got %f", got)
	}
}

func TestAddObservation_accumulates(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 2.0)
	ps.AddObservation("requests", 5.0)

	got := testutil.ToFloat64(ps.getCounter("requests", nil).With(prometheus.Labels{}))
	if got != 7.0 {
		t.Errorf("want 7.0, got %f", got)
	}
}

func TestMeasureTime_recordsHistogram(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.MeasureTime("latency", 300*time.Millisecond)
	ps.MeasureTime("latency", 700*time.Millisecond)

	gathered, err := ps.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range gathered {
		if mf.GetName() == "testns_latency" {
			h := mf.GetMetric()[0].GetHistogram()
			if h.GetSampleCount() != 2 {
				t.Errorf("want SampleCount 2, got %d", h.GetSampleCount())
			}
			if h.GetSampleSum() != 1.0 {
				t.Errorf("want SampleSum 1.0, got %f", h.GetSampleSum())
			}
			return
		}
	}
	t.Fatal("metric testns_latency not found")
}

func TestGauge_incrementThenDecrement(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.IncrementValue("connections", 5.0)
	ps.DecrementValue("connections", 2.0)

	got := testutil.ToFloat64(ps.getGauge("connections", nil).With(prometheus.Labels{}))
	if got != 3.0 {
		t.Errorf("want 3.0, got %f", got)
	}
}

func TestGauge_setValueOverwrites(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.IncrementValue("queue_depth", 10.0)
	ps.SetValue("queue_depth", 1.0)

	got := testutil.ToFloat64(ps.getGauge("queue_depth", nil).With(prometheus.Labels{}))
	if got != 1.0 {
		t.Errorf("want 1.0, got %f", got)
	}
}

func TestFixedLabels_appearOnMetric(t *testing.T) {
	ps := newTestPrometheusServer(t, Label{"env", "prod"})
	ps.AddObservation("requests", 1.0)

	gathered, err := ps.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range gathered {
		if mf.GetName() == "testns_requests" {
			for _, lp := range mf.GetMetric()[0].GetLabel() {
				if lp.GetName() == "env" && lp.GetValue() == "prod" {
					return
				}
			}
			t.Error("label env=prod not found on metric")
			return
		}
	}
	t.Fatal("metric testns_requests not found")
}

func TestLabelCollision_panics(t *testing.T) {
	ps := newTestPrometheusServer(t, Label{"env", "prod"})

	defer func() {
		if recover() == nil {
			t.Error("expected panic on label collision, got none")
		}
	}()

	ps.AddObservation("requests", 1.0, Label{"env", "dev"})
}

func TestSchemeLock_differentLabelKeysPanic(t *testing.T) {
	ps := newTestPrometheusServer(t)
	ps.AddObservation("requests", 1.0, Label{"endpoint", "/a"})

	defer func() {
		if recover() == nil {
			t.Error("expected panic on label schema change, got none")
		}
	}()

	ps.AddObservation("requests", 1.0, Label{"method", "GET"})
}

func TestPortConflict_returnsError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	_, err = newPrometheusServer("testns", port)
	if err == nil {
		t.Error("want error on occupied port, got nil")
	}
}
