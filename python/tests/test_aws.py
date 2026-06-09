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

import json
import unittest
from unittest.mock import MagicMock, patch

from spacearth.metrics.aws import AmazonCloudwatchMetricServer


def metric_key(name: str, labels=None) -> str:
    return json.dumps({"labels": labels, "name": name}, sort_keys=True)


class TestAWSGaugeAccumulation(unittest.TestCase):
    def setUp(self):
        patcher = patch("boto3.client", return_value=MagicMock())
        patcher.start()
        self.addCleanup(patcher.stop)
        self.server = AmazonCloudwatchMetricServer("testns", {})

    def _drain(self):
        self.server._queue.join()  # pylint: disable=protected-access

    def test_increment_accumulates(self):
        self.server.increment_value("gauge_inc", 5)
        self.server.increment_value("gauge_inc", 3)
        self._drain()
        self.assertEqual(self.server._last_values[metric_key("gauge_inc")], 8)  # pylint: disable=protected-access

    def test_decrement_reduces(self):
        self.server.increment_value("gauge_dec", 10)
        self.server.decrement_value("gauge_dec", 3)
        self._drain()
        self.assertEqual(self.server._last_values[metric_key("gauge_dec")], 7)  # pylint: disable=protected-access

    def test_set_value_overwrites(self):
        self.server.increment_value("gauge_set", 10)
        self.server.set_value("gauge_set", 1)
        self._drain()
        self.assertEqual(self.server._last_values[metric_key("gauge_set")], 1)  # pylint: disable=protected-access
