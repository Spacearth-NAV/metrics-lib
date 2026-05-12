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

// Package metrics provides a backend-agnostic interface for recording
// application metrics.
//
// The package is designed to be transparent inside a deployment: the backend
// is selected once at initialization, and all recording calls are identical
// regardless of which backend is active. Switching backends requires no
// changes to instrumentation code.
//
// # Backends
//
// Three backends are available:
//
//   - [AWS]: publishes to Amazon CloudWatch every 60 seconds. Requires AWS
//     credentials in the environment; see the AWS SDK documentation for the
//     full list of supported environment variables.
//   - [Prometheus]: exposes metrics at /metrics over HTTP. Starts a
//     background HTTP server on initialization. Requires a port via [WithPort]
//     (default 8080).
//   - [NoOp]: silently discards all metrics. No configuration required.
//     Useful in tests and local development.
//
// # Initialization
//
// [NewServer] is the only place where the backend is chosen:
//
//	srv, err := metrics.NewServer(
//	    metrics.Prometheus,
//	    "myapp",
//	    metrics.WithPort(9090),
//	    metrics.WithFixedLabels(metrics.Label{Key: "env", Value: "prod"}),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Fixed labels passed via [WithFixedLabels] are attached to every metric
// automatically, regardless of the backend in use.
//
// # Recording metrics
//
// All backends share the [Server] interface:
//
//	// counter
//	srv.AddObservation("requests_total", 1, metrics.Label{Key: "endpoint", Value: "/login"})
//
//	// histogram
//	srv.MeasureTime("request_duration", time.Since(start), metrics.Label{Key: "endpoint", Value: "/login"})
//
//	// gauge
//	srv.IncrementValue("active_connections", 1)
//	srv.DecrementValue("active_connections", 1)
//	srv.SetValue("queue_depth", float64(len(q)))
//
// # Prometheus label constraints
//
// The Prometheus backend locks the label schema for a metric on the first
// call. A subsequent call using a different set of label keys for the same
// metric name will panic. Call-site label keys must not overlap with fixed
// label keys; overlap also causes a panic.
package metrics
