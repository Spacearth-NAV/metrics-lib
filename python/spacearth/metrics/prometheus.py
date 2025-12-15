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

    __counters: dict[str, Counter]
    __histograms: dict[str, Histogram]
    __gauges: dict[str, Gauge]

    def __init__(self, namespace: str, fixed_labels: dict[str, str]):
        super().__init__(namespace, fixed_labels)
        start_http_server(8080)

    def __label_names(self, labels: Optional[dict[str, str]]):
        return [*(labels.keys() if labels is not None else []), *self._fixed_labels]

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        try:
            counter = self.__counters[name]
        except KeyError:
            counter = Counter(name, "", self.__label_names(labels), namespace=self._namespace)
            self.__counters[name] = counter
        if labels is not None:
            counter = counter.labels(**labels)
        counter = counter.labels(**self._fixed_labels)
        counter.inc(value)

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        try:
            histogram = self.__histograms[name]
        except KeyError:
            histogram = Histogram(name, "", self.__label_names(labels), namespace=self._namespace)
            self.__histograms[name] = histogram
        if labels is not None:
            histogram = histogram.labels(**labels)
        histogram = histogram.labels(**self._fixed_labels)
        histogram.observe(value)

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__label_names(labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        if labels is not None:
            gauge = gauge.labels(**labels)
        gauge = gauge.labels(**self._fixed_labels)
        gauge.inc(value)

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__label_names(labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        if labels is not None:
            gauge = gauge.labels(**labels)
        gauge = gauge.labels(**self._fixed_labels)
        gauge.dec(value)

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        try:
            gauge = self.__gauges[name]
        except KeyError:
            gauge = Gauge(name, "", self.__label_names(labels), namespace=self._namespace)
            self.__gauges[name] = gauge
        if labels is not None:
            gauge = gauge.labels(**labels)
        gauge = gauge.labels(**self._fixed_labels)
        gauge.set(value)
