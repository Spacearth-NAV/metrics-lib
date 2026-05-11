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
import socket
import unittest
from unittest.mock import patch

from prometheus_client import generate_latest  # type: ignore

from spacearth.metrics.prometheus import PrometheusMetricServer


class TestPrometheusMetricServer(unittest.TestCase):
    def setUp(self):
        patcher = patch("spacearth.metrics.prometheus.start_http_server")
        patcher.start()
        self.addCleanup(patcher.stop)
        self.server = PrometheusMetricServer("testns", {})
        self.registry = self.server._PrometheusMetricServer__registry  # pylint: disable=protected-access

    def _output(self) -> str:
        return generate_latest(self.registry).decode("utf-8")

    def test_add_observation_increments_counter(self):
        self.server.add_observation("requests", 3)
        self.assertIn("testns_requests_total 3.0", self._output())

    def test_add_observation_accumulates(self):
        self.server.add_observation("requests", 2)
        self.server.add_observation("requests", 5)
        self.assertIn("testns_requests_total 7.0", self._output())

    def test_measure_time_records_histogram(self):
        self.server.measure_time("latency", 0.5)
        self.server.measure_time("latency", 0.5)
        output = self._output()
        self.assertIn("testns_latency_count 2.0", output)
        self.assertIn("testns_latency_sum 1.0", output)

    def test_gauge_increment_then_decrement(self):
        self.server.increment_value("connections", 5)
        self.server.decrement_value("connections", 2)
        self.assertIn("testns_connections 3.0", self._output())

    def test_gauge_set_value_overwrites(self):
        self.server.increment_value("active", 10)
        self.server.set_value("active", 1)
        self.assertIn("testns_active 1.0", self._output())

    def test_fixed_labels_appear_on_all_metrics(self):
        server = PrometheusMetricServer("testns2", {"env": "prod"})
        registry = server._PrometheusMetricServer__registry  # pylint: disable=protected-access
        server.add_observation("events", 1)
        output = generate_latest(registry).decode("utf-8")
        self.assertIn('env="prod"', output)
        self.assertIn("testns2_events_total", output)

    def test_label_collision_raises_value_error(self):
        server = PrometheusMetricServer("testns3", {"env": "prod"})
        with self.assertRaises(ValueError) as ctx:
            server.add_observation("requests", 1, labels={"env": "dev"})
        self.assertIn("env", str(ctx.exception))


class TestPrometheusPortConflict(unittest.TestCase):
    def test_port_conflict_raises_runtime_error(self):
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            sock.bind(("", 0))
            sock.listen(1)
            port = sock.getsockname()[1]
            with self.assertRaises(RuntimeError):
                PrometheusMetricServer("ns", {}, port=port)
        finally:
            sock.close()
