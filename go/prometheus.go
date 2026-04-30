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
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type prometheusServer struct {
	namespace   string
	fixedLabels []Label
	registry    *prometheus.Registry

	lock       sync.RWMutex
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec

	server *http.Server
}

// NewPrometheusServer creates a Prometheus metric server that exposes metrics
// at /metrics on the given port. Fixed labels are added to every metric.
// Use NewServer with WithPort and WithFixedLabels to read the port from the
// METRICS_PROMETHEUS_PORT environment variable instead.
func NewPrometheusServer(namespace string, port int, fixedLabels ...Label) (Server, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
	}

	s := &prometheusServer{
		namespace:   namespace,
		fixedLabels: fixedLabels,
		registry:    prometheus.NewRegistry(),
		counters:    make(map[string]*prometheus.CounterVec),
		histograms:  make(map[string]*prometheus.HistogramVec),
		gauges:      make(map[string]*prometheus.GaugeVec),
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{Registry: s.registry}))
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("prometheus http server stopped unexpectedly", "error", err)
		}
	}()

	logger.Info(fmt.Sprintf("created Prometheus metric server on port %d", port))
	return s, nil
}

func (p *prometheusServer) labelNames(labels []Label) []string {
	res := make([]string, 0, len(labels)+len(p.fixedLabels))
	for _, l := range labels {
		res = append(res, l.Key)
	}
	for _, l := range p.fixedLabels {
		res = append(res, l.Key)
	}
	return res
}

func (p *prometheusServer) labelValues(labels []Label) prometheus.Labels {
	res := make(prometheus.Labels, len(labels)+len(p.fixedLabels))
	for _, l := range labels {
		res[l.Key] = l.Value
	}
	for _, l := range p.fixedLabels {
		res[l.Key] = l.Value
	}
	return res
}

// getOrCreate returns the cached metric for name, or calls create to register
// it on first use. It uses a read lock for the fast path (metric already
// exists) so concurrent calls for different metrics do not block each other.
// A write lock is acquired only when registration is needed, with a
// double-check to handle the race between releasing the read lock and
// acquiring the write lock.
func getOrCreate[T any](lock *sync.RWMutex, cache map[string]*T, name string, create func() *T) *T {
	lock.RLock()
	if v, ok := cache[name]; ok {
		lock.RUnlock()
		return v
	}
	lock.RUnlock()

	lock.Lock()
	defer lock.Unlock()

	if v, ok := cache[name]; ok {
		return v
	}

	v := create()
	cache[name] = v
	return v
}

func (p *prometheusServer) getCounter(name string, labels []Label) *prometheus.CounterVec {
	return getOrCreate(&p.lock, p.counters, name, func() *prometheus.CounterVec {
		c := prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.labelNames(labels))
		if err := p.registry.Register(c); err != nil {
			panic(fmt.Errorf("developer error: failed to register prometheus counter %q: %w", name, err))
		}
		return c
	})
}

func (p *prometheusServer) getHistogram(name string, labels []Label) *prometheus.HistogramVec {
	return getOrCreate(&p.lock, p.histograms, name, func() *prometheus.HistogramVec {
		h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.labelNames(labels))
		if err := p.registry.Register(h); err != nil {
			panic(fmt.Errorf("developer error: failed to register prometheus histogram %q: %w", name, err))
		}
		return h
	})
}

func (p *prometheusServer) getGauge(name string, labels []Label) *prometheus.GaugeVec {
	return getOrCreate(&p.lock, p.gauges, name, func() *prometheus.GaugeVec {
		g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: p.namespace,
			Name:      name,
		}, p.labelNames(labels))
		if err := p.registry.Register(g); err != nil {
			panic(fmt.Errorf("developer error: failed to register prometheus gauge %q: %w", name, err))
		}
		return g
	})
}

func (p *prometheusServer) AddObservation(name string, value float64, labels ...Label) {
	c := p.getCounter(name, labels)
	c.With(p.labelValues(labels)).Add(value)
}

func (p *prometheusServer) MeasureTime(name string, value time.Duration, labels ...Label) {
	h := p.getHistogram(name, labels)
	h.With(p.labelValues(labels)).Observe(value.Seconds())
}

func (p *prometheusServer) IncrementValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	g.With(p.labelValues(labels)).Add(value)
}

func (p *prometheusServer) DecrementValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	g.With(p.labelValues(labels)).Sub(value)
}

func (p *prometheusServer) SetValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	g.With(p.labelValues(labels)).Set(value)
}