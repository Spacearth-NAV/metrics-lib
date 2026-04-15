"""Demo application generating synthetic metrics for both Prometheus and CloudWatch backends."""

import logging
import os
import random
import time

from spacearth.metrics import MetricServer

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")

# Create both metric servers — same interface, different backends
prometheus_server = MetricServer.create_server("prometheus", "myapp", {"environment": "demo"})

if os.environ.get("AWS_ENDPOINT_URL"):
    CLOUDWATCH_SERVER = MetricServer.create_server("aws", "myapp", {"environment": "demo"})
    logging.info("CloudWatch server enabled via moto")
else:
    CLOUDWATCH_SERVER = None
    logging.info("CloudWatch server disabled (set AWS_ENDPOINT_URL to enable)")

ENDPOINTS = ["/login", "/api/users", "/api/orders", "/health"]
STEPS = ["auth", "process", "serialize"]


def record_observation(name, value, labels=None):
    """Record a counter observation to all active backends."""
    prometheus_server.add_observation(name, value, labels=labels)
    if CLOUDWATCH_SERVER:
        CLOUDWATCH_SERVER.add_observation(name, value, labels=labels)


def record_time(name, value, labels=None):
    """Record a timing observation to all active backends."""
    prometheus_server.measure_time(name, value, labels=labels)
    if CLOUDWATCH_SERVER:
        CLOUDWATCH_SERVER.measure_time(name, value, labels=labels)


def record_increment(name, value=1, labels=None):
    """Increment a gauge on all active backends."""
    prometheus_server.increment_value(name, value, labels=labels)
    if CLOUDWATCH_SERVER:
        CLOUDWATCH_SERVER.increment_value(name, value, labels=labels)


def record_decrement(name, value=1, labels=None):
    """Decrement a gauge on all active backends."""
    prometheus_server.decrement_value(name, value, labels=labels)
    if CLOUDWATCH_SERVER:
        CLOUDWATCH_SERVER.decrement_value(name, value, labels=labels)


def record_set(name, value, labels=None):
    """Set a gauge value on all active backends."""
    prometheus_server.set_value(name, value, labels=labels)
    if CLOUDWATCH_SERVER:
        CLOUDWATCH_SERVER.set_value(name, value, labels=labels)


def simulate():
    """Simulate a single request processing cycle with random metrics."""
    endpoint = random.choice(ENDPOINTS)
    record_observation("requests_received", 1, labels={"endpoint": endpoint, "method": "GET"})
    record_observation("requests_received", 1, labels={"endpoint": "/ciccio", "method": "POST"})
    #record_observation("requests_received", 1, labels={"endpoint": "/ciccio"})
    # Measure processing time
    t_start = time.time()
    time.sleep(random.uniform(0.01, 0.2))
    t_end = time.time()
    step = random.choice(STEPS)
    record_time("processing_time", t_end - t_start, labels={"step": step})

    # Track active connections
    if random.random() < 0.3:
        record_increment("active_connections", labels={"endpoint": endpoint})
        time.sleep(random.uniform(0.05, 0.1))
        record_decrement("active_connections", labels={"endpoint": endpoint})

    # Update current users gauge
    record_set("current_users", random.randint(10, 100))


if __name__ == "__main__":
    logging.info("Starting metrics demo...")
    logging.info("Prometheus:  http://localhost:8080/metrics")
    if CLOUDWATCH_SERVER:
        logging.info("CloudWatch:   moto at %s", os.environ["AWS_ENDPOINT_URL"])
    while True:
        simulate()
        time.sleep(1)
