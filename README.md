# Metrics library

This repo contains a metrics library implemented in Go and Python.
The goal of the library is to be transparent inside the deployment: if metrics are disabled, the application code does not change.

## Installation

### Python

```bash
pip install spacearth-metrics
```

### Go

```bash
go get github.com/Spacearth-NAV/metrics-lib
```

## Usage

### Configuration

Each backend requires specific configuration before the server is initialized.

#### AWS

Set the following environment variables before starting your application:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_DEFAULT_REGION`

Refer to the [AWS SDK configuration documentation](https://docs.aws.amazon.com/sdkref/latest/guide/environment-variables.html) for more info.

#### Prometheus

The Prometheus backend starts an HTTP server that exposes metrics at `/metrics`.

- **Python**: port is set via the `port` keyword argument (default `8080`).
- **Go**: port is required and must be set explicitly via `WithPort`.

> **Note:** Prometheus requires all label names for a metric to be declared upfront. The label schema is locked on the first call for each metric name. Any subsequent call with a different set of label keys will raise an error. Make sure to use the same label keys consistently across all calls to the same metric.

#### No-op

No configuration required. All calls are silently ignored.

---

### Initialization

The library is designed to be transparent: initialization is the only place where the backend is chosen. All metric recording calls are identical regardless of the backend in use.

**Python**

```python
import os
from spacearth.metrics import MetricServer

provider    = os.getenv("METRIC_PROVIDER", "aws")
namespace   = os.getenv("METRIC_NAMESPACE", "default")
environment = os.getenv("ENVIRONMENT", "development")

labels     = {"environment": environment}
extra_args = {}

# Prometheus requires a port
if provider == "prometheus":
    extra_args["port"] = int(os.getenv("PROMETHEUS_PORT", "8080"))

metric_server = MetricServer.create_server(provider, namespace, labels, **extra_args)
# AWS:        set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION
# Prometheus: set PROMETHEUS_PORT (default: 8080)
# No-op:      no configuration required
```

**Go**

```go
import (
    "os"
    "strconv"

    metrics "github.com/Spacearth-NAV/metrics-lib/go"
)

func envOr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

provider    := envOr("METRIC_PROVIDER", "aws")
namespace   := envOr("METRIC_NAMESPACE", "default")
environment := envOr("ENVIRONMENT", "development")

opts := []metrics.Option{
    metrics.WithFixedLabels(metrics.Label{"environment", environment}),
}

// Prometheus requires a port
if provider == "prometheus" {
    port, _ := strconv.Atoi(envOr("PROMETHEUS_PORT", "8080"))
    opts = append(opts, metrics.WithPort(port))
}

metricsServer, err := metrics.NewServer(metrics.ServerType(provider), namespace, opts...)
if err != nil {
    // handle error
}
// AWS:        set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION
// Prometheus: set PROMETHEUS_PORT (default: 8080)
// No-op:      no configuration required
```

---

### Recording metrics

All backends share the same interface. Fixed labels passed at initialization are automatically added to every metric.

#### Counters — `add_observation` / `AddObservation`

Records a single event count.

**Python**

```python
metric_server.add_observation("requests_received", 1, labels={"endpoint": "/login"})
```

**Go**

```go
metricsServer.AddObservation("requests_received", 1, metrics.Label{"endpoint", "/login"})
```

#### Histograms — `measure_time` / `MeasureTime`

Records a duration. Python accepts seconds as a `float`; Go accepts a `time.Duration`.

**Python**

```python
import time

t_start = time.time()
# ... do work ...
metric_server.measure_time("processing_time", time.time() - t_start, labels={"step": "auth"})
```

**Go**

```go
start := time.Now()
// ... do work ...
metricsServer.MeasureTime("processing_time", time.Since(start), metrics.Label{"step", "auth"})
```

#### Gauges — `increment_value` / `decrement_value` / `set_value`

Tracks a value that goes up and down.

**Python**

```python
def on_connection(conn):
    metric_server.increment_value("active_connections", labels={"endpoint": "/ws"})
    try:
        while conn.connected:
            pass
    finally:
        metric_server.decrement_value("active_connections", labels={"endpoint": "/ws"})

# or set an absolute value
metric_server.set_value("queue_depth", 42)
```

**Go**

```go
metricsServer.IncrementValue("active_connections", 1, metrics.Label{"endpoint", "/ws"})
metricsServer.DecrementValue("active_connections", 1, metrics.Label{"endpoint", "/ws"})

metricsServer.SetValue("queue_depth", 42)
```
