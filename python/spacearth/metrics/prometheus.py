# Copyright 2025 Spacearth NAV S.r.l.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
This file provides a Prometheus-based metric server.
"""

from threading import Lock
from typing import Optional

from prometheus_client import CollectorRegistry, Counter, Gauge, Histogram, start_http_server  # type: ignore

from .metric_server import MetricServer


class PrometheusMetricServer(MetricServer):
    """
    Exposes metrics via a prometheus server.
    """

    __counters: dict[str, Counter]
    __histograms: dict[str, Histogram]
    __gauges: dict[str, Gauge]

    def __init__(self, namespace: str, fixed_labels: dict[str, str], *, port: int = 8080):
        super().__init__(namespace, fixed_labels)
        self.__lock = Lock()
        self.__registry = CollectorRegistry()
        self.__counters: dict[str, Counter] = {}
        self.__histograms: dict[str, Histogram] = {}
        self.__gauges: dict[str, Gauge] = {}
        try:
            start_http_server(port, registry=self.__registry)
        except OSError as e:
            raise RuntimeError(f"failed to start Prometheus HTTP server on port {port}") from e

    def __merged_labels(self, labels: Optional[dict[str, str]]) -> dict[str, str]:
        if labels is not None:
            collision = set(labels.keys()) & set(self._fixed_labels.keys())
            if collision:
                raise ValueError(f"label keys conflict with fixed_labels: {collision}")
        return {**(labels or {}), **self._fixed_labels}

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        with self.__lock:
            if name not in self.__counters:
                self.__counters[name] = Counter(
                    name, "", list(merged.keys()), namespace=self._namespace, registry=self.__registry
                )
        self.__counters[name].labels(**merged).inc(value)

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        with self.__lock:
            if name not in self.__histograms:
                self.__histograms[name] = Histogram(
                    name, "", list(merged.keys()), namespace=self._namespace, registry=self.__registry
                )
        self.__histograms[name].labels(**merged).observe(value)

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        with self.__lock:
            if name not in self.__gauges:
                self.__gauges[name] = Gauge(
                    name, "", list(merged.keys()), namespace=self._namespace, registry=self.__registry
                )
        self.__gauges[name].labels(**merged).inc(value)

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        with self.__lock:
            if name not in self.__gauges:
                self.__gauges[name] = Gauge(
                    name, "", list(merged.keys()), namespace=self._namespace, registry=self.__registry
                )
        self.__gauges[name].labels(**merged).dec(value)

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        with self.__lock:
            if name not in self.__gauges:
                self.__gauges[name] = Gauge(
                    name, "", list(merged.keys()), namespace=self._namespace, registry=self.__registry
                )
        self.__gauges[name].labels(**merged).set(value)
