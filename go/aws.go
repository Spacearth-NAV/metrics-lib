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
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

type metricInfo struct {
	name   string
	unit   string
	labels []Label
}

type metricData struct {
	metricInfo
	action    string
	timestamp time.Time
	value     float64
}

type awsCloudWatchServer struct {
	namespace   string
	fixedLabels []Label

	client *cloudwatch.CloudWatch

	data   chan metricData
	cancel context.CancelFunc

	metricLock   sync.Mutex
	metrics      map[string]metricInfo
	lastValues   map[string]float64
	observations map[string]map[time.Time][]float64
}

func newAmazonCloudwatchServer(namespace string, fixedLabels ...Label) (Server, error) {
	res := &awsCloudWatchServer{
		namespace:    namespace,
		lastValues:   make(map[string]float64),
		metrics:      make(map[string]metricInfo),
		observations: make(map[string]map[time.Time][]float64),
		fixedLabels:  fixedLabels,
		data:         make(chan metricData),
	}

	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS SDK session: %w", err)
	}

	res.client = cloudwatch.New(sess)

	var ctx context.Context
	ctx, res.cancel = context.WithCancel(context.Background())

	//go res.publishMetrics(ctx)
	go res.gatherMetrics(ctx)
	go res.exportMetrics(ctx)

	return res, nil
}

func (a *awsCloudWatchServer) dimensions(m metricInfo) []*cloudwatch.Dimension {
	res := make([]*cloudwatch.Dimension, 0, len(m.labels)+len(a.fixedLabels))

	for _, l := range m.labels {
		res = append(res, &cloudwatch.Dimension{
			Name:  aws.String(l.Key),
			Value: aws.String(l.Value),
		})
	}

	for _, l := range a.fixedLabels {
		res = append(res, &cloudwatch.Dimension{
			Name:  aws.String(l.Key),
			Value: aws.String(l.Value),
		})
	}

	return res
}

func (a *awsCloudWatchServer) metricIdentifier(m metricInfo) string {
	identifierMap := make(map[string]string)
	identifierMap["name"] = m.name
	for _, l := range m.labels {
		identifierMap[l.Key] = l.Value
	}
	// this cannot fail
	idBytes, _ := json.Marshal(identifierMap)
	return string(idBytes)
}

func (a *awsCloudWatchServer) publishMetrics(ctx context.Context) {
	for {
		select {
		case d := <-a.data:
			value := d.value

			metricId := a.metricIdentifier(d.metricInfo)

			switch d.action {
			case "increment_value":
				a.lastValues[metricId] += value
				value = a.lastValues[metricId]
			case "decrement_value":
				a.lastValues[metricId] -= value
				value = a.lastValues[metricId]
			case "set_value":
				a.lastValues[metricId] = value
				value = a.lastValues[metricId]
			}

			_, err := a.client.PutMetricDataWithContext(ctx, &cloudwatch.PutMetricDataInput{
				Namespace: aws.String(a.namespace),
				MetricData: []*cloudwatch.MetricDatum{
					{
						MetricName: aws.String(d.name),
						Dimensions: a.dimensions(d.metricInfo),
						Timestamp:  aws.Time(d.timestamp),
						Unit:       aws.String(d.unit),
						Value:      aws.Float64(value),
					},
				},
			})

			if err != nil {
				logger.Error("failed to publish metric data", "error", err)
			}
		case <-ctx.Done():
			logger.Debug("Stopping publish metrics loop")
			return
		}
	}
}

func (a *awsCloudWatchServer) gatherMetrics(ctx context.Context) {
	for {
		select {
		case d := <-a.data:
			logger.Debug(fmt.Sprintf("received metric %s", d.name), "metric", d)

			metricId := a.metricIdentifier(d.metricInfo)
			timestamp := d.timestamp.Truncate(time.Minute).Add(time.Minute)

			if _, ok := a.metrics[metricId]; !ok {
				a.metrics[metricId] = d.metricInfo
			}

			if _, ok := a.observations[metricId]; !ok {
				a.observations[metricId] = make(map[time.Time][]float64)
			}

			if _, ok := a.observations[metricId][timestamp]; !ok {
				a.observations[metricId][timestamp] = make([]float64, 0)
			}

			value := d.value

			switch d.action {
			case "increment_value":
				logger.Debug(fmt.Sprintf("incrementing metric value by %f", value))
				a.lastValues[metricId] += value
				value = a.lastValues[metricId]
			case "decrement_value":
				logger.Debug(fmt.Sprintf("decrementing metric value by %f", value))
				a.lastValues[metricId] -= value
				value = a.lastValues[metricId]
			case "set_value":
				logger.Debug(fmt.Sprintf("setting metric value to %f", value))
				a.lastValues[metricId] = value
				value = a.lastValues[metricId]
			}

			a.observations[metricId][timestamp] = append(a.observations[metricId][timestamp], value)

		case <-ctx.Done():
			return
		}
	}
}

func getCounts(in []float64) (counts []float64, values []float64) {
	tmp := make(map[float64]int)

	for _, v := range in {
		tmp[v] += 1
	}

	counts = make([]float64, 0, len(tmp))
	values = make([]float64, 0, len(tmp))

	for k, v := range tmp {
		counts = append(counts, float64(v))
		values = append(values, k)
	}

	return
}

func (a *awsCloudWatchServer) exportMetrics(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case now := <-ticker.C:
			now = now.UTC().Truncate(time.Minute)

			data := make([]*cloudwatch.MetricDatum, 0)

			a.metricLock.Lock()
			for metricId, info := range a.metrics {
				foundOneObservation := false

				dataByTimestamp := a.observations[metricId]
				for timestamp, observations := range dataByTimestamp {
					if timestamp.After(now) {
						continue
					}

					counts, values := getCounts(observations)

					data = append(data, &cloudwatch.MetricDatum{
						Dimensions: a.dimensions(info),
						MetricName: aws.String(info.name),
						Timestamp:  aws.Time(now),
						Unit:       aws.String(info.unit),
						Values:     aws.Float64Slice(values),
						Counts:     aws.Float64Slice(counts),
					})

					foundOneObservation = true
					delete(dataByTimestamp, timestamp)
				}

				if value, ok := a.lastValues[metricId]; !foundOneObservation && ok {
					data = append(data, &cloudwatch.MetricDatum{
						Dimensions: a.dimensions(info),
						MetricName: aws.String(info.name),
						Timestamp:  aws.Time(now),
						Unit:       aws.String(info.unit),
						Value:      aws.Float64(value),
					})
				}
			}
			a.metricLock.Unlock()

			if len(data) > 0 {
				_, err := a.client.PutMetricDataWithContext(ctx, &cloudwatch.PutMetricDataInput{
					Namespace:  aws.String(a.namespace),
					MetricData: data,
				})

				if err != nil {
					logger.Error("failed to publish metric data", "error", err)
				} else {
					logger.Info(fmt.Sprintf("published all metrics up to %s", now))
				}
			} else {
				logger.Info(fmt.Sprintf("no metrics to publish up to %s", now))
			}

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (a *awsCloudWatchServer) AddObservation(name string, value float64, labels ...Label) {
	a.data <- metricData{
		metricInfo: metricInfo{
			name:   name,
			unit:   "Count",
			labels: labels,
		},
		action:    "add_observation",
		timestamp: time.Now().UTC(),
		value:     value,
	}
}

func (a *awsCloudWatchServer) MeasureTime(name string, value time.Duration, labels ...Label) {
	a.data <- metricData{
		metricInfo: metricInfo{
			name:   name,
			unit:   "Seconds",
			labels: labels,
		},
		action:    "add_observation",
		timestamp: time.Now().UTC(),
		value:     value.Seconds(),
	}
}

func (a *awsCloudWatchServer) IncrementValue(name string, value float64, labels ...Label) {
	a.data <- metricData{
		metricInfo: metricInfo{
			name:   name,
			unit:   "Count",
			labels: labels,
		},
		action:    "increment_value",
		timestamp: time.Now().UTC(),
		value:     value,
	}
}

func (a *awsCloudWatchServer) DecrementValue(name string, value float64, labels ...Label) {
	a.data <- metricData{
		metricInfo: metricInfo{
			name:   name,
			unit:   "Count",
			labels: labels,
		},
		action:    "decrement_value",
		timestamp: time.Now().UTC(),
		value:     value,
	}
}

func (a *awsCloudWatchServer) SetValue(name string, value float64, labels ...Label) {
	a.data <- metricData{
		metricInfo: metricInfo{
			name:   name,
			unit:   "Count",
			labels: labels,
		},
		action:    "set_value",
		timestamp: time.Now().UTC(),
		value:     value,
	}
}
