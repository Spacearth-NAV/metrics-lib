# Python Metrics Example

This example demonstrates how to use the `spacearth-metrics` library with both **Prometheus** and **AWS CloudWatch** as backends, showing that they are fully interchangeable through the same interface.

The demo application generates synthetic metrics every second and sends them to both backends simultaneously.

## Architecture

```
                        ┌─────────────────┐
                        │     app.py      │
                        │  (metrics demo) │
                        └────────┬────────┘
                                 │
                    ┌────────────┼────────────┐
                    │                         │
                    ▼                         ▼
           ┌──────────────┐          ┌──────────────┐
           │  Prometheus  │          │   moto       │
           │  :8080       │          │   :5000      │
           └──────┬───────┘          │ (CloudWatch  │
                  │                  │   mock)      │
           ┌──────┴───────┐          └──────────────┘
           │              │
           ▼              ▼
    ┌────────────┐ ┌──────────┐
    │ Prometheus │ │  Grafana │
    │ :9090      │ │  :3000   │
    └────────────┘ └──────────┘
```

## Quick Start

```bash
docker compose up --build
```

Once all services are up, open:

| Service | URL | Description |
|---------|-----|-------------|
| Metrics endpoint | http://localhost:8080/metrics | Raw Prometheus metrics |
| Prometheus | http://localhost:9090 | Query and explore metrics |
| Grafana | http://localhost:3000 | Pre-configured dashboard (anonymous login) |

## Metrics Generated

The demo application generates the following metrics under the `myapp` namespace, with a fixed label `environment=demo`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `requests_received` | Counter | `endpoint` | Total requests per endpoint |
| `processing_time` | Histogram | `step` | Request processing latency in seconds |
| `active_connections` | Gauge | `endpoint` | Currently active connections |
| `current_users` | Gauge | — | Current number of users |

### Example PromQL Queries

```promql
# Request rate per second
rate(myapp_requests_received_total[1m])

# 95th percentile latency
histogram_quantile(0.95, rate(myapp_processing_time_bucket[5m]))

# Total active connections
sum(myapp_active_connections)
```

## Querying CloudWatch Metrics

The CloudWatch backend is mocked using [moto](https://github.com/getmoto/moto), a Python library that emulates AWS services. You can query the stored metrics from within the app container:

```bash
# List all registered metrics
docker compose exec app python /app/query_cloudwatch.py list

# Get statistics for a specific metric
docker compose exec app python /app/query_cloudwatch.py stats requests_received

# Show stats for all metrics
docker compose exec app python /app/query_cloudwatch.py all
```

## Plug-and-Play

The key feature of this library is that switching between backends requires only changing a single string:

```python
from spacearth.metrics import MetricServer

# Prometheus backend
server = MetricServer.create_server("prometheus", "myapp", {"environment": "demo"})

# AWS CloudWatch backend
server = MetricServer.create_server("aws", "myapp", {"environment": "demo"})

# No-op (metrics are silently discarded)
server = MetricServer.create_server("noop", "myapp", {"environment": "demo"})
```

All backends share the same interface — `add_observation`, `measure_time`, `increment_value`, `decrement_value`, `set_value` — so your application code does not need to change when you switch providers.

## File Structure

```
examples/python/
├── app.py                    # Demo application
├── query_cloudwatch.py       # Script to query CloudWatch metrics from moto
├── Dockerfile                # App container (Python + prometheus-python + boto3)
├── Dockerfile.moto           # Moto server container (CloudWatch mock)
├── docker-compose.yml        # Orchestrates all services
├── prometheus.yml            # Prometheus scrape configuration
└── grafana/
    └── provisioning/
        ├── datasources/
        │   └── prometheus.yml    # Auto-configured Prometheus datasource
        └── dashboards/
            ├── dashboard.yml     # Dashboard provider config
            └── dashboard.json    # Pre-built dashboard with 6 panels
```

## Teardown

```bash
docker compose down
```

To remove persisted Grafana data:

```bash
docker compose down -v
```
