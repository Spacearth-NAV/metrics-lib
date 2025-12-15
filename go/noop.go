// Copyright 2025 Spacearth NAV S.r.l.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"fmt"
	"time"
)

type noOpServer struct{}

func (n *noOpServer) AddObservation(name string, value float64, labels ...Label) {
	logger.Debug(fmt.Sprintf("observed value %f for metric %s", value, name), "labels", labels)
}

func (n *noOpServer) MeasureTime(name string, value time.Duration, labels ...Label) {
	logger.Debug(fmt.Sprintf("observed value %v for metric %s", value, name), "labels", labels)
}

func (n *noOpServer) IncrementValue(name string, value float64, labels ...Label) {
	logger.Debug(fmt.Sprintf("incrementing metric %s by %f", name, value), "labels", labels)
}

func (n *noOpServer) DecrementValue(name string, value float64, labels ...Label) {
	logger.Debug(fmt.Sprintf("decrementing metric %s by %f", name, value), "labels", labels)
}

func (n *noOpServer) SetValue(name string, value float64, labels ...Label) {
	logger.Debug(fmt.Sprintf("setting metric %s to value %f", name, value), "labels", labels)
}
