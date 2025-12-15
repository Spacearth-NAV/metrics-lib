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

type Label struct {
	Key   string
	Value string
}

type ServerType string

const (
	AWS  ServerType = "aws"
	NoOp ServerType = "noop"
)

type Server interface {
	AddObservation(name string, value float64, labels ...Label)
	MeasureTime(name string, value time.Duration, labels ...Label)
	IncrementValue(name string, value float64, labels ...Label)
	DecrementValue(name string, value float64, labels ...Label)
	SetValue(name string, value float64, labels ...Label)
}

func NewServer(serverType ServerType, namespace string, fixedLabels ...Label) (Server, error) {
	var res Server = &noOpServer{}

	switch serverType {
	case AWS:
		srv, err := newAmazonCloudwatchServer(namespace, fixedLabels...)
		if err != nil {
			logger.Error("failed to create Amazon Cloudwatch metric server", "error", err)
			return nil, err
		} else {
			logger.Info("created Amazon Cloudwatch metric server")
			res = srv
		}
	case NoOp:
		logger.Info("created placeholder (No-Op) metric server: no metrics will be published")
	default:
		logger.Warn(fmt.Sprintf("unknown server type %s", serverType))
		logger.Warn("defaulting to placeholder (No-Op) metric server: no metrics will be published")
	}

	return res, nil
}
