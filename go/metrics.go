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
	"fmt"
	"time"
)

type Label struct {
	Key   string
	Value string
}

type ServerType string

const (
	AWS        ServerType = "aws"
	Prometheus ServerType = "prometheus"
	NoOp       ServerType = "noop"
)

type Server interface {
	AddObservation(name string, value float64, labels ...Label)
	MeasureTime(name string, value time.Duration, labels ...Label)
	IncrementValue(name string, value float64, labels ...Label)
	DecrementValue(name string, value float64, labels ...Label)
	SetValue(name string, value float64, labels ...Label)
}

// Option configures the server created by NewServer.
type Option func(*serverConfig)

type serverConfig struct {
	fixedLabels []Label
	port        int
}

// WithFixedLabels adds labels that are attached to every metric regardless of backend.
func WithFixedLabels(labels ...Label) Option {
	return func(c *serverConfig) {
		c.fixedLabels = append(c.fixedLabels, labels...)
	}
}

// WithPort sets the TCP port for backends that expose an HTTP server (Prometheus).
// Required when using NewServer with the Prometheus backend.
func WithPort(port int) Option {
	return func(c *serverConfig) { c.port = port }
}

func NewServer(serverType ServerType, namespace string, opts ...Option) (Server, error) {
	cfg := &serverConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	var res Server = &noOpServer{}

	switch serverType {
	case AWS:
		srv, err := newAmazonCloudwatchServer(namespace, cfg.fixedLabels...)
		if err != nil {
			logger.Error("failed to create Amazon Cloudwatch metric server", "error", err)
			return nil, err
		}
		logger.Info("created Amazon Cloudwatch metric server")
		res = srv
	case Prometheus:
		srv, err := newPrometheusServer(namespace, cfg.port, cfg.fixedLabels...)
		if err != nil {
			logger.Error("failed to create Prometheus metric server", "error", err)
			return nil, err
		}
		res = srv
	case NoOp:
		logger.Info("created placeholder (No-Op) metric server: no metrics will be published")
	default:
		logger.Warn(fmt.Sprintf("unknown server type %s", serverType))
		logger.Warn("defaulting to placeholder (No-Op) metric server: no metrics will be published")
	}

	return res, nil
}
