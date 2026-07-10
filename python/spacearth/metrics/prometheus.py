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

from prometheus_client import CollectorRegistry, Counter, Gauge, Histogram, start_http_server

from .metric_server import MetricServer


class PrometheusMetricServer(MetricServer):
    """
    Exposes metrics via a prometheus server.
    """

    __counters: dict[str, Counter]
    __histograms: dict[str, Histogram]
    __gauges: dict[str, Gauge]

    def __init__(
        self,
        namespace: str,
        fixed_labels: dict[str, str],
        *,
        port: int = 8080,
        registry: Optional[CollectorRegistry] = None,
    ):
        super().__init__(namespace, fixed_labels)
        self.__lock = Lock()
        self.__registry = registry if registry is not None else CollectorRegistry()
        self.__counters = {}
        self.__histograms = {}
        self.__gauges = {}
        try:
            start_http_server(port, registry=self.__registry)
        except OSError as e:
            raise RuntimeError(f"failed to start Prometheus HTTP server on port {port}") from e

    def __get_or_create_metric(self, name: str, merged_labels: dict[str, str], cache: dict, metric_class):
        try:
            metric = cache[name]
        except KeyError:
            with self.__lock:
                try:
                    metric = cache[name]
                except KeyError:
                    metric = metric_class(
                        name, "", list(merged_labels.keys()), namespace=self._namespace, registry=self.__registry
                    )
                    cache[name] = metric
        return metric.labels(**merged_labels) if merged_labels else metric

    def __merged_labels(self, labels: Optional[dict[str, str]]) -> dict[str, str]:
        if labels is not None:
            collision = set(labels.keys()) & set(self._fixed_labels.keys())
            if collision:
                raise ValueError(f"label keys conflict with fixed_labels: {collision}")
        return {**(labels or {}), **self._fixed_labels}

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        self.__get_or_create_metric(name, merged, self.__counters, Counter).inc(value)

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        self.__get_or_create_metric(name, merged, self.__histograms, Histogram).observe(value)

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        self.__get_or_create_metric(name, merged, self.__gauges, Gauge).inc(value)

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        self.__get_or_create_metric(name, merged, self.__gauges, Gauge).dec(value)

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        merged = self.__merged_labels(labels)
        self.__get_or_create_metric(name, merged, self.__gauges, Gauge).set(value)
