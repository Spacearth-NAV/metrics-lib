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
This file provides the interface for metric server implementations.
"""

import logging
from abc import ABC, abstractmethod
from typing import Optional


class MetricServer(ABC):
    """
    Abstract class defining the interface for the metric server.
    """

    def __init__(self, namespace: str, fixed_labels: dict[str, str]):
        self._namespace = namespace
        self._fixed_labels = fixed_labels

    @abstractmethod
    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        """
        Adds an observation to the metric named `name`.

        :param name: The name of the metric.
        :param value: The value to record.
        :param labels: Custom labels to add to the metric.
        """

    @abstractmethod
    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        """
        Records a duration in seconds (the `value`) inside the metric named `name`.

        :param name: The name of the metric.
        :param value: The duration (in seconds) to record.
        :param labels: Custom labels to add to the metric.
        """

    @abstractmethod
    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        """
        Increments a persistent metric by the given `value`.

        :param name: The name of the metric.
        :param value: The value to increase the metric by (default 1).
        :param labels: Custom labels to add to the metric.
        """

    @abstractmethod
    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        """
        Decrements a persistent metric by the given `value`.

        :param name: The name of the metric.
        :param value: The value to decrease the metric by (default 1).
        :param labels: Custom labels to add to the metric.
        """

    @abstractmethod
    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        """
        Sets a persistent metric to the given `value`.

        :param name: The name of the metric.
        :param value: The value to set the metric to.
        :param labels: Custom labels to add to the metric.
        """

    @classmethod
    def create_server(cls, server_type: str, namespace: str, fixed_labels: dict[str, str]) -> "MetricServer":
        """
        Factory method to create an instance of a metric server with the given type.
        Defaults to the NoOp metric server.

        :param server_type: The type of the server.
        :param namespace: The namespace to apply to all metrics.
        :param fixed_labels: The set of fixed labels to add to all metrics
        :return: An instance of the metric server.
        """
        # pylint: disable=import-outside-toplevel,cyclic-import
        from .noop import NoOpMetricServer

        server: "MetricServer" = NoOpMetricServer(namespace, fixed_labels)

        if server_type == "aws":
            from .aws import AmazonCloudwatchMetricServer

            logging.info("creating an Amazon Cloudwatch metric server")
            server = AmazonCloudwatchMetricServer(namespace, fixed_labels)

        elif server_type == "prometheus":
            from .prometheus import PrometheusMetricServer

            logging.info("creating a Prometheus metric server")
            server = PrometheusMetricServer(namespace, fixed_labels)

        elif server_type == "noop":
            logging.info("using placeholder (No-Op) metric server: metrics will not be exported")

        else:
            logging.warning("unknown metric server type %s", server_type)
            logging.warning("defaulting to a placeholder (No-Op) metric server: metrics will not be exported")

        return server
