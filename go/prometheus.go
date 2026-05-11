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
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPrometheusPort = 8080

type prometheusServer struct {
	namespace   string
	fixedLabels []Label
	registry    *prometheus.Registry

	counters   sync.Map // string -> *prometheus.CounterVec
	histograms sync.Map // string -> *prometheus.HistogramVec
	gauges     sync.Map // string -> *prometheus.GaugeVec

	server *http.Server
}

// newPrometheusServer creates a Prometheus metric server that exposes metrics
// at /metrics on the given port. Fixed labels are added to every metric.
func newPrometheusServer(namespace string, port int, fixedLabels ...Label) (Server, error) {
	s := &prometheusServer{
		namespace:   namespace,
		fixedLabels: fixedLabels,
		registry:    prometheus.NewRegistry(),
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{Registry: s.registry}))

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("prometheus: port %d unavailable: %w", port, err)
	}

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("prometheus http server stopped unexpectedly", "error", err)
		}
	}()

	logger.Info(fmt.Sprintf("created Prometheus metric server on port %d", port))
	return s, nil
}

// checkLabelCollision panics if any call-site label key matches a fixed label key,
// mirroring the ValueError raised by the Python implementation.
func (p *prometheusServer) checkLabelCollision(labels []Label) {
	fixed := make(map[string]struct{}, len(p.fixedLabels))
	for _, l := range p.fixedLabels {
		fixed[l.Key] = struct{}{}
	}
	for _, l := range labels {
		if _, ok := fixed[l.Key]; ok {
			panic(fmt.Sprintf("metrics: label key %q conflicts with a fixed label", l.Key))
		}
	}
}

func (p *prometheusServer) mergedLabelNames(labels []Label) []string {
	res := make([]string, 0, len(labels)+len(p.fixedLabels))
	for _, l := range labels {
		res = append(res, l.Key)
	}
	for _, l := range p.fixedLabels {
		res = append(res, l.Key)
	}
	return res
}

func (p *prometheusServer) mergedLabelValues(labels []Label) prometheus.Labels {
	res := make(prometheus.Labels, len(labels)+len(p.fixedLabels))
	for _, l := range labels {
		res[l.Key] = l.Value
	}
	for _, l := range p.fixedLabels {
		res[l.Key] = l.Value
	}
	return res
}

// getOrCreate returns the cached metric for name, or registers a new one on
// first use. sync.Map.LoadOrStore is atomic: only the goroutine that wins the
// store calls MustRegister; any concurrent goroutine gets the stored instance
// and skips registration entirely.
func getOrCreate[T prometheus.Collector](reg prometheus.Registerer, m *sync.Map, name string, create func() T) T {
	if v, ok := m.Load(name); ok {
		return v.(T)
	}
	c := create()
	actual, loaded := m.LoadOrStore(name, c)
	if !loaded {
		reg.MustRegister(c)
	}
	return actual.(T)
}

func (p *prometheusServer) getCounter(name string, labels []Label) *prometheus.CounterVec {
	return getOrCreate(p.registry, &p.counters, name, func() *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.mergedLabelNames(labels))
	})
}

func (p *prometheusServer) getHistogram(name string, labels []Label) *prometheus.HistogramVec {
	return getOrCreate(p.registry, &p.histograms, name, func() *prometheus.HistogramVec {
		return prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.mergedLabelNames(labels))
	})
}

func (p *prometheusServer) getGauge(name string, labels []Label) *prometheus.GaugeVec {
	return getOrCreate(p.registry, &p.gauges, name, func() *prometheus.GaugeVec {
		return prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.mergedLabelNames(labels))
	})
}

func (p *prometheusServer) AddObservation(name string, value float64, labels ...Label) {
	p.checkLabelCollision(labels)
	p.getCounter(name, labels).With(p.mergedLabelValues(labels)).Add(value)
}

func (p *prometheusServer) MeasureTime(name string, value time.Duration, labels ...Label) {
	p.checkLabelCollision(labels)
	p.getHistogram(name, labels).With(p.mergedLabelValues(labels)).Observe(value.Seconds())
}

func (p *prometheusServer) IncrementValue(name string, value float64, labels ...Label) {
	p.checkLabelCollision(labels)
	p.getGauge(name, labels).With(p.mergedLabelValues(labels)).Add(value)
}

func (p *prometheusServer) DecrementValue(name string, value float64, labels ...Label) {
	p.checkLabelCollision(labels)
	p.getGauge(name, labels).With(p.mergedLabelValues(labels)).Sub(value)
}

func (p *prometheusServer) SetValue(name string, value float64, labels ...Label) {
	p.checkLabelCollision(labels)
	p.getGauge(name, labels).With(p.mergedLabelValues(labels)).Set(value)
}
