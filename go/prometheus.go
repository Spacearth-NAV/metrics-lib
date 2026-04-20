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
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultPrometheusPort = 8080
	prometheusPortEnvVar  = "METRICS_PROMETHEUS_PORT"
)

// prometheusPortFromEnv returns the TCP port the Prometheus HTTP server should
// bind to. It reads the METRICS_PROMETHEUS_PORT environment variable and falls
// back to defaultPrometheusPort if the variable is unset, empty, or not a
// valid port number.
func prometheusPortFromEnv() int {
	raw, ok := os.LookupEnv(prometheusPortEnvVar)
	if !ok || raw == "" {
		return defaultPrometheusPort
	}

	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		logger.Warn(fmt.Sprintf("invalid %s=%q, falling back to default port %d", prometheusPortEnvVar, raw, defaultPrometheusPort))
		return defaultPrometheusPort
	}
	return port
}

type prometheusServer struct {
	namespace   string
	fixedLabels []Label
	registry    *prometheus.Registry

	lock         sync.Mutex
	counters     map[string]*prometheus.CounterVec
	histograms   map[string]*prometheus.HistogramVec
	gauges       map[string]*prometheus.GaugeVec
	metricLabels map[string]map[string]struct{}

	server *http.Server
}

// NewPrometheusServer creates a metric server that exposes metrics in the
// Prometheus text format under the "/metrics" path. The TCP port is read from
// the METRICS_PROMETHEUS_PORT environment variable and defaults to 8080 when
// the variable is unset or invalid. The server uses a dedicated
// prometheus.Registry, so the exposition only contains metrics produced
// through this Server instance.
func NewPrometheusServer(namespace string, fixedLabels ...Label) (Server, error) {
	return newPrometheusServer(namespace, prometheusPortFromEnv(), fixedLabels...)
}

func newPrometheusServer(namespace string, port int, fixedLabels ...Label) (*prometheusServer, error) {
	res := buildPrometheusServer(namespace, fixedLabels)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(res.registry, promhttp.HandlerOpts{Registry: res.registry}))
	res.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := res.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("prometheus http server stopped unexpectedly", "error", err)
		}
	}()

	return res, nil
}

// buildPrometheusServer constructs a prometheusServer with a dedicated registry
// but does not start the HTTP server. Useful for tests that want to observe
// the registry directly without binding a port.
func buildPrometheusServer(namespace string, fixedLabels []Label) *prometheusServer {
	return &prometheusServer{
		namespace:    namespace,
		fixedLabels:  fixedLabels,
		registry:     prometheus.NewRegistry(),
		counters:     make(map[string]*prometheus.CounterVec),
		histograms:   make(map[string]*prometheus.HistogramVec),
		gauges:       make(map[string]*prometheus.GaugeVec),
		metricLabels: make(map[string]map[string]struct{}),
	}
}

// labelSet returns the combined set of label keys for a per-call labels slice
// plus the server's fixed labels. Used for order-insensitive validation.
func (p *prometheusServer) labelSet(labels []Label) map[string]struct{} {
	res := make(map[string]struct{}, len(labels)+len(p.fixedLabels))
	for _, l := range labels {
		res[l.Key] = struct{}{}
	}
	for _, l := range p.fixedLabels {
		res[l.Key] = struct{}{}
	}
	return res
}

// sameLabelSet reports whether two label-key sets contain exactly the same keys.
func sameLabelSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// checkLabelSet verifies that the incoming label keys match the set registered
// for `name` at first use. On mismatch it logs and returns false so the caller
// can skip the operation instead of letting prometheus panic on an inconsistent
// cardinality. Must be called with p.lock held; the metric is assumed to be
// already registered (and therefore present in p.metricLabels).
func (p *prometheusServer) checkLabelSet(name string, incoming map[string]struct{}) bool {
	registered := p.metricLabels[name]
	if sameLabelSet(registered, incoming) {
		return true
	}
	logger.Error("label set mismatch for metric; dropping value to avoid panic",
		"name", name,
		"registered", setKeys(registered),
		"got", setKeys(incoming),
	)
	return false
}

func setKeys(s map[string]struct{}) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
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

func (p *prometheusServer) getCounter(name string, labels []Label) *prometheus.CounterVec {
	p.lock.Lock()
	defer p.lock.Unlock()

	incoming := p.labelSet(labels)

	if c, ok := p.counters[name]; ok {
		if !p.checkLabelSet(name, incoming) {
			return nil
		}
		return c
	}

	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: p.namespace,
		Name:      name,
	}, p.labelNames(labels))
	if err := p.registry.Register(c); err != nil {
		logger.Error("failed to register prometheus counter", "name", name, "error", err)
		return nil
	}

	p.counters[name] = c
	p.metricLabels[name] = incoming
	return c
}

func (p *prometheusServer) getHistogram(name string, labels []Label) *prometheus.HistogramVec {
	p.lock.Lock()
	defer p.lock.Unlock()

	incoming := p.labelSet(labels)

	if h, ok := p.histograms[name]; ok {
		if !p.checkLabelSet(name, incoming) {
			return nil
		}
		return h
	}

	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: p.namespace,
		Name:      name,
	}, p.labelNames(labels))
	if err := p.registry.Register(h); err != nil {
		logger.Error("failed to register prometheus histogram", "name", name, "error", err)
		return nil
	}

	p.histograms[name] = h
	p.metricLabels[name] = incoming
	return h
}

func (p *prometheusServer) getGauge(name string, labels []Label) *prometheus.GaugeVec {
	p.lock.Lock()
	defer p.lock.Unlock()

	incoming := p.labelSet(labels)

	if g, ok := p.gauges[name]; ok {
		if !p.checkLabelSet(name, incoming) {
			return nil
		}
		return g
	}

	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: p.namespace,
		Name:      name,
	}, p.labelNames(labels))
	if err := p.registry.Register(g); err != nil {
		logger.Error("failed to register prometheus gauge", "name", name, "error", err)
		return nil
	}

	p.gauges[name] = g
	p.metricLabels[name] = incoming
	return g
}

func (p *prometheusServer) AddObservation(name string, value float64, labels ...Label) {
	c := p.getCounter(name, labels)
	if c == nil {
		return
	}
	c.With(p.labelValues(labels)).Add(value)
}

func (p *prometheusServer) MeasureTime(name string, value time.Duration, labels ...Label) {
	h := p.getHistogram(name, labels)
	if h == nil {
		return
	}
	h.With(p.labelValues(labels)).Observe(value.Seconds())
}

func (p *prometheusServer) IncrementValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	if g == nil {
		return
	}
	g.With(p.labelValues(labels)).Add(value)
}

func (p *prometheusServer) DecrementValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	if g == nil {
		return
	}
	g.With(p.labelValues(labels)).Sub(value)
}

func (p *prometheusServer) SetValue(name string, value float64, labels ...Label) {
	g := p.getGauge(name, labels)
	if g == nil {
		return
	}
	g.With(p.labelValues(labels)).Set(value)
}
