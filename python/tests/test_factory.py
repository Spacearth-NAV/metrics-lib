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

# pylint: disable=missing-function-docstring,missing-module-docstring,missing-class-docstring

import unittest
from unittest.mock import patch

from spacearth.metrics.metric_server import MetricServer
from spacearth.metrics.noop import NoOpMetricServer
from spacearth.metrics.prometheus import PrometheusMetricServer


class TestFactory(unittest.TestCase):
    def test_noop_type_returns_noop(self):
        server = MetricServer.create_server("noop", "ns", {})
        self.assertIsInstance(server, NoOpMetricServer)

    def test_unknown_type_defaults_to_noop(self):
        server = MetricServer.create_server("invalid", "ns", {})
        self.assertIsInstance(server, NoOpMetricServer)

    @patch("spacearth.metrics.prometheus.start_http_server")
    def test_prometheus_type_returns_prometheus_server(self, _mock):
        server = MetricServer.create_server("prometheus", "ns", {}, port=9090)
        self.assertIsInstance(server, PrometheusMetricServer)

    @patch("spacearth.metrics.prometheus.start_http_server")
    def test_prometheus_passes_fixed_labels(self, _mock):
        server = MetricServer.create_server("prometheus", "ns", {"env": "prod"}, port=9091)
        self.assertIsInstance(server, PrometheusMetricServer)
        self.assertEqual(server._fixed_labels, {"env": "prod"})  # pylint: disable=protected-access

    def test_noop_passes_fixed_labels(self):
        server = MetricServer.create_server("noop", "ns", {"env": "prod"})
        self.assertEqual(server._fixed_labels, {"env": "prod"})  # pylint: disable=protected-access
