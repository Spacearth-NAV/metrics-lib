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
This file provides a metric server that does nothing.
"""

from typing import Optional

from .metric_server import MetricServer


class NoOpMetricServer(MetricServer):
    """
    The NoOpMetricServer does nothing when a metric is published.
    """

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        pass

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        pass

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        pass

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        pass

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        pass
