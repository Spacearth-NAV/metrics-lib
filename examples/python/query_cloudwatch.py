"""Query CloudWatch metrics from moto server.

Usage:
    docker compose exec app python /app/query_cloudwatch.py list
    docker compose exec app python /app/query_cloudwatch.py stats requests_received
    docker compose exec app python /app/query_cloudwatch.py all
"""

import os
import sys
from datetime import datetime, timedelta, timezone

import boto3


def get_client() -> boto3.client:
    """Create a CloudWatch client pointing to the moto server."""
    return boto3.client(
        "cloudwatch",
        endpoint_url=os.environ.get("AWS_ENDPOINT_URL", "http://localhost:5000"),
        region_name=os.environ.get("AWS_DEFAULT_REGION", "us-east-1"),
    )


def list_metrics(client: boto3.client):
    """List all metrics in the myapp namespace."""
    print("\n=== Metrics in namespace 'myapp' ===\n")
    resp = client.list_metrics(Namespace="myapp")
    if not resp["Metrics"]:
        print("  (no metrics found)")
        return
    for m in resp["Metrics"]:
        dims = ", ".join(f'{d["Name"]}={d["Value"]}' for d in m.get("Dimensions", []))
        print(f"  {m['MetricName']:<25s} [{dims}]")


def get_stats(client: boto3.client, metric_name: str):
    """Get and display statistics for a specific metric."""
    print(f"\n=== Statistics for '{metric_name}' ===\n")
    end = datetime.now(timezone.utc)
    start = end - timedelta(minutes=5)

    stats = ["Sum", "Average", "Maximum", "Minimum", "SampleCount"]
    try:
        resp = client.get_metric_statistics(
            Namespace="myapp",
            MetricName=metric_name,
            StartTime=start,
            EndTime=end,
            Period=60,
            Statistics=stats,
        )
    except client.exceptions.MetricNotFound:
        print(f"  Metric '{metric_name}' not found")
        return

    if not resp["Datapoints"]:
        print("  (no datapoints in the last 5 minutes)")
        return

    # Sort by timestamp descending
    for dp in sorted(resp["Datapoints"], key=lambda x: x["Timestamp"], reverse=True):
        ts = dp["Timestamp"].strftime("%H:%M:%S")
        values = " | ".join(f"{s}={dp.get(s, '-'):>8}" for s in stats)
        print(f"  {ts}  {values}")


def show_all(client: boto3.client):
    """Show statistics for all metrics in the namespace."""
    resp = client.list_metrics(Namespace="myapp")
    names = sorted(set(m["MetricName"] for m in resp["Metrics"]))
    for name in names:
        get_stats(client, name)


def main():
    """CLI entry point for querying CloudWatch metrics."""
    client = get_client()
    cmd = sys.argv[1] if len(sys.argv) > 1 else "list"

    if cmd == "list":
        list_metrics(client)
    elif cmd == "stats":
        if len(sys.argv) < 3:
            print("Usage: query_cloudwatch.py stats <metric_name>")
            sys.exit(1)
        get_stats(client, sys.argv[2])
    elif cmd == "all":
        show_all(client)
    else:
        print(f"Unknown command: {cmd}")
        print("Available: list, stats <name>, all")
        sys.exit(1)


if __name__ == "__main__":
    main()
