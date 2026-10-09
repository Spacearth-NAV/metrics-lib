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
from unittest.mock import MagicMock, patch

from spacearth.metrics.aws import AmazonCloudwatchMetricServer


class TestAWSGaugeAccumulation(unittest.TestCase):
    def setUp(self):
        self.mock_client = MagicMock()
        patcher = patch("boto3.client", return_value=self.mock_client)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.server = AmazonCloudwatchMetricServer("testns", {})

    def _last_published_value(self, metric_name: str) -> float | None:
        """Return the Value from the most recent keepalive put_metric_data call for metric_name."""
        for call in reversed(self.mock_client.put_metric_data.call_args_list):
            for datum in call.kwargs["MetricData"]:
                if datum["MetricName"] == metric_name and "Value" in datum:
                    return datum["Value"]
        return None

    def test_increment_accumulates(self):
        self.server.increment_value("gauge_inc", 5)
        self.server.increment_value("gauge_inc", 3)
        self.server.flush()  # gather + export stat set {5, 8}
        self.server.flush()  # export keepalive with final accumulated value
        self.assertEqual(self._last_published_value("gauge_inc"), 8)

    def test_decrement_reduces(self):
        self.server.increment_value("gauge_dec", 10)
        self.server.decrement_value("gauge_dec", 3)
        self.server.flush()
        self.server.flush()
        self.assertEqual(self._last_published_value("gauge_dec"), 7)

    def test_set_value_overwrites(self):
        self.server.increment_value("gauge_set", 10)
        self.server.set_value("gauge_set", 1)
        self.server.flush()
        self.server.flush()
        self.assertEqual(self._last_published_value("gauge_set"), 1)


class TestAWSPublishLimits(unittest.TestCase):
    def setUp(self):
        self.mock_client = MagicMock()
        patcher = patch("boto3.client", return_value=self.mock_client)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.server = AmazonCloudwatchMetricServer("testns", {})

    def _published_requests(self) -> list[list[dict]]:
        """Return the MetricData list of every put_metric_data call, in order."""
        return [call.kwargs["MetricData"] for call in self.mock_client.put_metric_data.call_args_list]

    def _published_datums(self, metric_name: str) -> list[dict]:
        """Return every published datum for metric_name, across all calls."""
        return [datum for data in self._published_requests() for datum in data if datum["MetricName"] == metric_name]

    def test_distinct_values_are_split_across_datums(self):
        expected = {i / 1000 for i in range(200)}
        for value in expected:
            self.server.measure_time("latency", value)
        self.server.flush()

        datums = self._published_datums("latency")

        self.assertGreater(len(datums), 1, "200 distinct values must not fit in a single datum")
        for datum in datums:
            self.assertLessEqual(len(datum["Values"]), AmazonCloudwatchMetricServer.MAX_VALUES)

        published = {value for datum in datums for value in datum["Values"]}
        self.assertEqual(published, expected)
        self.assertEqual(sum(sum(datum["Counts"]) for datum in datums), 200)

    def test_datums_are_split_across_requests(self):
        total = AmazonCloudwatchMetricServer.MAX_METRICS + 1
        for i in range(total):
            self.server.add_observation(f"metric_{i}", 1)
        self.server.flush()

        requests = self._published_requests()

        self.assertEqual(len(requests), 2)
        for data in requests:
            self.assertLessEqual(len(data), AmazonCloudwatchMetricServer.MAX_METRICS)
        self.assertEqual(sum(len(data) for data in requests), total)
