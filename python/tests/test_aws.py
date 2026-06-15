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
