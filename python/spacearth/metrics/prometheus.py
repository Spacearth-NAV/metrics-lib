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

from typing import Optional

from prometheus_client import Counter, Gauge, Histogram, start_http_server  # type: ignore

from .metric_server import MetricServer


class PrometheusMetricServer(MetricServer):
    """
    Exposes metrics via a prometheus server.
    """

    # Prometheus Collectors keyed by metric name.
    __counters: dict[str, Counter]
    __histograms: dict[str, Histogram]
    __gauges: dict[str, Gauge]

    # Tracks the dynamic label keys registered for each metric.
    # Prometheus requires all label names to be declared at Collector creation time,
    # so we lock in the label schema on the first call for each metric name.
    # Subsequent calls with unregistered label keys are silently filtered out.
    __registered_labels: dict[str, list[str]]

    def __init__(self, namespace: str, fixed_labels: dict[str, str]):
        super().__init__(namespace, fixed_labels)
        self.__counters: dict[str, Counter] = {}
        self.__histograms: dict[str, Histogram] = {}
        self.__gauges: dict[str, Gauge] = {}
        self.__registered_labels: dict[str, list[str]] = {}
        start_http_server(8080)

    def __get_label_names(self, name: str, labels: Optional[dict[str, str]]) -> list[str]:
        """Return the full list of label names for a metric (dynamic + fixed).
        On the first call for a given metric name, the dynamic label keys are
        captured and stored. Subsequent calls reuse the same label schema.
        """
        if name not in self.__registered_labels:
            self.__registered_labels[name] = sorted((labels or {}).keys())
        return [*self.__registered_labels[name], *self._fixed_labels]

    def __active_labels(self, name: str, labels: Optional[dict[str, str]]) -> dict[str, str]:
        """Merge dynamic labels (filtered to registered keys) with fixed labels.
        Applying all labels in a single ``.labels()`` call ensures a consistent
        label tuple order across all recordings of the same metric.
        """
        registered = self.__registered_labels.get(name, [])
        dynamic = {k: v for k, v in (labels or {}).items() if k in registered}
        return {**dynamic, **self._fixed_labels}

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        try:
            counter = self.__counters[name]
        except KeyError:
            counter = Counter(name, "", self.__get_label_names(name, labels), namespace=self._namespace)
            self.__counters[name] = counter
        counter.labels(**self.__active_labels(name, labels)).inc(value)

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        try:
            histogram = self.__histograms[name]
        except KeyError:
            histogram = Histogram(name, "", self.__get_label_names(name, labels), namespace=self._namespace)
            self.__histograms[name] = histogram
        histogram.labels(**self.__active_labels(name, labels)).observe(value)

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__get_label_names(name, labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        gauge.labels(**self.__active_labels(name, labels)).inc(value)

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__get_label_names(name, labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        gauge.labels(**self.__active_labels(name, labels)).dec(value)

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__get_label_names(name, labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        gauge.labels(**self.__active_labels(name, labels)).set(value)
