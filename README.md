# Metrics library

This repo contains a metrics library implemented in Go and Python.
The goal of the metric library is to be transparent inside the deployment if metrics are disabled.

When using the Amazon CloudWatch metric server, configure the following environment variables:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_DEFAULT_REGION`

Refer to the [AWS SDK configuration documentation](https://docs.aws.amazon.com/sdkref/latest/guide/environment-variables.html) for more info.

## Installation

For python:

```bash
pip install spacearth-metrics
```

For go:

```bash
go get github.com/Spacearth-NAV/metrics-lib
```

## Usage

For the python library:

```python
import time

from spacearth.metrics import MetricServer

# create the server, add some fixed labels
metric_server = MetricServer.create_server("aws", "custom", {"environment": "development"})

# increment counters
metric_server.add_observation("requests_received", 1, labels={"endpoint": "/login"})

# measure times
t_start = time.time()
# do something
t_end = time.time()
metric_server.measure_time("processing_time", t_end - t_start, labels={"step": "something"})


# keep track of resources
def on_connection(conn):
    metric_server.increment_value("connections")
    try:
        while conn.connected:
            # handle connection
            pass
    finally:
        metric_server.decrement_value("connections")
```

For the go library:

```go
package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Spacearth-NAV/metrics-lib/go"
)

var metricsServer metrics.Server

func main() {
	// you can specify a logger: the interface is compatible with the slog package
	metrics.SetLogger(slog.Default())
	// instantiate the metric server
	var err error
	metricsServer, err = metrics.NewServer(metrics.AWS, "custom", metrics.Label{"environment", "development"})
	if err != nil {
		panic(err)
    }

	http.HandleFunc("GET /info", connectionCallback)
}

func connectionCallback(w http.ResponseWriter, r *http.Request) {
	// count active connections
	metricsServer.IncrementValue("active_connections", 1, metrics.Label{"endpoint", "info"})
	defer metricsServer.DecrementValue("active_connections", 1, metrics.Label{"endpoint", "info"})

	// count requests
	metricsServer.AddObservation("received_requests", 1, metrics.Label{"endpoint", "info"})

	// measure time
	start := time.Now()

	/* do something */

	end := time.Now()
	metricsServer.MeasureTime("processing_time", end.Sub(start), metrics.Label{"endpoint", "info"})

	w.WriteHeader(http.StatusOK)
}
```
