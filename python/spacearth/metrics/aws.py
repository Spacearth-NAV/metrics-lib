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
Provides a metric server based on Amazon CloudWatch.
"""

import json
import logging
import time
from collections import Counter, defaultdict
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from queue import Queue
from threading import Lock, Thread
from typing import Literal, Optional

import boto3

from .metric_server import MetricServer


@dataclass
class MetricData:
    """
    Metric data DTO.
    """

    action: str
    unit: Literal["Count", "Seconds"]
    timestamp: datetime
    name: str
    value: float | int
    labels: Optional[dict[str, str]]


@dataclass
class MetricInfo:
    """
    Contains metric information, including name, unit, and label.
    """

    name: str
    unit: Literal["Count", "Seconds"]
    labels: dict[str, str]


class AmazonCloudwatchMetricServer(MetricServer):
    """
    Publishes metrics to Amazon CloudWatch every 60 seconds.
    Metrics are published on a separate thread.
    """

    __queue: Queue = Queue()
    __metrics_lock: Lock = Lock()

    __metrics: dict[str, MetricInfo] = {}
    __observations: dict[str, dict[datetime, list[float | int]]] = defaultdict(lambda: defaultdict(list))
    __last_values: dict[str, float | int] = defaultdict(int)

    def __init__(self, namespace: str, fixed_labels: dict[str, str]):
        super().__init__(namespace, fixed_labels)
        self.__logger = logging.getLogger(self.__class__.__name__)
        self.__client = boto3.client("cloudwatch")

        # self.__publish_thread = Thread(target=self.__publish_loop, daemon=True)
        # self.__publish_thread.start()

        self.__gather_thread = Thread(target=self.__gather_loop, daemon=True)
        self.__export_thread = Thread(target=self.__export_loop, daemon=True)
        self.__gather_thread.start()
        self.__export_thread.start()

    # pylint: disable-next=unused-private-member
    def __publish_loop(self) -> None:
        while True:
            try:
                self.__logger.debug("Waiting for next metric...")
                metric: MetricData = self.__queue.get()
                self.__logger.debug("Received metric %s", metric)
                try:
                    labels = self._fixed_labels | (metric.labels if metric.labels is not None else {})
                    value = metric.value

                    metric_identifier = json.dumps({"name": metric.name, "labels": metric.labels}, sort_keys=True)

                    if metric.action == "increment_value":
                        self.__last_values[metric_identifier] += metric.value
                        value = self.__last_values[metric_identifier]
                    elif metric.action == "decrement_value":
                        self.__last_values[metric_identifier] -= metric.value
                        value = self.__last_values[metric_identifier]
                    elif metric.action == "set_value":
                        self.__last_values[metric_identifier] = metric.value
                        value = self.__last_values[metric_identifier]

                    self.__client.put_metric_data(
                        Namespace=self._namespace,
                        MetricData=[
                            {
                                "MetricName": metric.name,
                                "Timestamp": metric.timestamp.astimezone(timezone.utc),
                                "Value": value,
                                "Unit": metric.unit,
                                "Dimensions": [{"Name": k, "Value": v} for k, v in labels.items()],
                            }
                        ],
                    )
                except Exception:  # pylint: disable=broad-exception-caught
                    self.__logger.exception("Failed to publish metrics")
            finally:
                self.__queue.task_done()

    def __gather_loop(self) -> None:
        while True:
            try:
                self.__logger.debug("Waiting for next metric...")
                metric: MetricData = self.__queue.get()
                self.__logger.debug("Received metric %s", metric)

                with self.__metrics_lock:
                    metric_identifier = json.dumps({"name": metric.name, "labels": metric.labels}, sort_keys=True)
                    metric_timestamp = metric.timestamp.replace(second=0, microsecond=0) + timedelta(minutes=1)

                    if metric_identifier not in self.__metrics:
                        labels = self._fixed_labels | (metric.labels if metric.labels is not None else {})
                        self.__metrics[metric_identifier] = MetricInfo(metric.name, metric.unit, labels)

                    current_metric = self.__observations[metric_identifier][metric_timestamp]

                    if metric.action == "add_observation":
                        self.__logger.debug("Adding value %s to metric %s", metric.value, metric_identifier)
                        current_metric.append(metric.value)
                    else:
                        if metric.action == "increment_value":
                            self.__logger.debug("Incrementing metric %s by %s", metric_identifier, metric.value)
                            self.__last_values[metric_identifier] += metric.value
                        elif metric.action == "decrement_value":
                            self.__logger.debug("Decrementing metric %s by %s", metric_identifier, metric.value)
                            self.__last_values[metric_identifier] -= metric.value
                        elif metric.action == "set_value":
                            self.__logger.debug("Setting metric %s to %s", metric_identifier, metric.value)
                            self.__last_values[metric_identifier] = metric.value

                        current_metric.append(self.__last_values[metric_identifier])

                    self.__logger.info("Metric %s stored", metric)
            finally:
                self.__queue.task_done()

    def __export_loop(self) -> None:
        while True:
            sleep_until = datetime.now().replace(second=0, microsecond=0) + timedelta(minutes=1)
            sleep_for = (sleep_until - datetime.now()).total_seconds()

            self.__logger.info("Waiting for %f seconds before publishing metrics...", sleep_for)
            time.sleep(sleep_for)

            self.__logger.info("Publishing metrics...")
            with self.__metrics_lock:
                data_to_publish = []

                for metric_name, metric in self.__metrics.items():
                    observations = [t for t in self.__observations[metric_name].keys() if t <= sleep_until]

                    for timestamp in observations:
                        values, counts = zip(*Counter(self.__observations[metric_name][timestamp]).items())
                        data_to_publish.append(
                            {
                                "MetricName": metric.name,
                                "Timestamp": timestamp.astimezone(timezone.utc),
                                "Values": values,
                                "Counts": counts,
                                "Unit": metric.unit,
                                "Dimensions": [{"Name": k, "Value": v} for k, v in metric.labels.items()],
                            }
                        )
                        del self.__observations[metric_name][timestamp]

                    if not observations and metric_name in self.__last_values:
                        data_to_publish.append(
                            {
                                "MetricName": metric.name,
                                "Timestamp": datetime.now(timezone.utc).replace(second=0, microsecond=0),
                                "Value": self.__last_values[metric_name],
                                "Unit": metric.unit,
                                "Dimensions": [{"Name": k, "Value": v} for k, v in metric.labels.items()],
                            }
                        )

            try:
                if len(data_to_publish) > 0:
                    self.__client.put_metric_data(Namespace=self._namespace, MetricData=data_to_publish)  # type: ignore

                    self.__logger.info("All metrics published up to %s", sleep_until)
                else:
                    self.__logger.info("No metrics to publish up to %s", sleep_until)
            except Exception:  # pylint: disable=broad-exception-caught
                self.__logger.exception("Failed to publish metrics")

    def add_observation(self, name: str, value: int, labels: Optional[dict[str, str]] = None):
        self.__queue.put(MetricData("add_observation", "Count", datetime.now(), name, value, labels))

    def measure_time(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        self.__queue.put(MetricData("add_observation", "Seconds", datetime.now(), name, value, labels))

    def increment_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        self.__queue.put(MetricData("increment_value", "Count", datetime.now(), name, value, labels))

    def decrement_value(self, name: str, value: float = 1, labels: Optional[dict[str, str]] = None):
        self.__queue.put(MetricData("decrement_value", "Count", datetime.now(), name, value, labels))

    def set_value(self, name: str, value: float, labels: Optional[dict[str, str]] = None):
        self.__queue.put(MetricData("set_value", "Count", datetime.now(), name, value, labels))
