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

### Initialization

#### AWS

Set the following environment variables before starting your application:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_DEFAULT_REGION`

Refer to the [AWS SDK configuration documentation](https://docs.aws.amazon.com/sdkref/latest/guide/environment-variables.html) for more info.

**Python**

```python
from spacearth.metrics import MetricServer

metric_server = MetricServer.create_server("aws", "my_namespace", {"environment": "production"})
```

**Go**

```go
metricsServer, err = metrics.NewServer(metrics.AWS, "my_namespace", metrics.Label{"environment", "production"})
```

#### Prometheus

The Prometheus backend starts an HTTP server that exposes metrics at `/metrics`. The port defaults to `8080` and can be changed via the `port` keyword argument.

> **Note:** Prometheus requires all label names for a metric to be declared upfront. The label schema is locked on the first call for each metric name. Any subsequent call with a different set of label keys will raise an error from the underlying `prometheus_client` library. Make sure to use the same label keys consistently across all calls to the same metric.

**Python**

```python
from spacearth.metrics import MetricServer

# default port 8080
metric_server = MetricServer.create_server("prometheus", "my_namespace", {"environment": "production"})

# custom port
metric_server = MetricServer.create_server("prometheus", "my_namespace", {"environment": "production"}, port=9090)
```

**Go**

Go support for the Prometheus backend is tracked in a separate PR.

#### No-op

When metrics are disabled (e.g. in local development), use the `noop` backend. All calls are silently ignored.

```python
metric_server = MetricServer.create_server("noop", "my_namespace", {})
```

---

### Recording metrics

All backends share the same interface. Fixed labels passed at initialization are automatically added to every metric.

#### Counters — `add_observation`

Records a single event count.

```python
metric_server.add_observation("requests_received", 1, labels={"endpoint": "/login"})
```

#### Histograms — `measure_time`

Records a duration in seconds.

```python
import time

t_start = time.time()
# ... do work ...
metric_server.measure_time("processing_time", time.time() - t_start, labels={"step": "auth"})
```

#### Gauges — `increment_value`, `decrement_value`, `set_value`

Tracks a value that goes up and down.

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
metricsServer.AddObservation("requests_received", 1, metrics.Label{"endpoint", "/login"})

metricsServer.MeasureTime("processing_time", time.Since(start), metrics.Label{"step", "auth"})

metricsServer.IncrementValue("active_connections", 1, metrics.Label{"endpoint", "/ws"})
metricsServer.DecrementValue("active_connections", 1, metrics.Label{"endpoint", "/ws"})
```
