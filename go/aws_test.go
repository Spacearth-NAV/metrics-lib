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
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCloudWatch records the MetricData of every request it receives.
type stubCloudWatch struct {
	requests [][]*cloudwatch.MetricDatum
}

func (s *stubCloudWatch) PutMetricDataWithContext(
	_ aws.Context,
	input *cloudwatch.PutMetricDataInput,
	_ ...request.Option,
) (*cloudwatch.PutMetricDataOutput, error) {
	s.requests = append(s.requests, input.MetricData)

	return &cloudwatch.PutMetricDataOutput{}, nil
}

// newTestServer returns a server publishing to stub, with no goroutine running:
// tests populate the pending observations and call doExport directly.
func newTestServer(stub *stubCloudWatch) *awsCloudWatchServer {
	return &awsCloudWatchServer{
		namespace:    "testns",
		client:       stub,
		metrics:      make(map[string]metricInfo),
		lastValues:   make(map[string]float64),
		observations: make(map[string]map[time.Time][]float64),
	}
}

func TestExport_splitsDistinctValuesAcrossDatums(t *testing.T) {
	stub := &stubCloudWatch{}
	srv := newTestServer(stub)
	now := time.Now().UTC().Truncate(time.Minute)

	observations := make([]float64, 0, 200)
	for i := 0; i < 200; i++ {
		observations = append(observations, float64(i))
	}

	srv.metrics["latency_id"] = metricInfo{name: "latency", unit: "Seconds"}
	srv.observations["latency_id"] = map[time.Time][]float64{now: observations}

	srv.doExport(context.Background(), now)

	require.Len(t, stub.requests, 1)

	datums := stub.requests[0]
	require.Greater(t, len(datums), 1, "200 distinct values must not fit in a single datum")

	published := make(map[float64]bool, 200)
	total := 0.0

	for _, datum := range datums {
		values := aws.Float64ValueSlice(datum.Values)
		assert.LessOrEqual(t, len(values), maxValues)

		for _, v := range values {
			published[v] = true
		}

		for _, c := range aws.Float64ValueSlice(datum.Counts) {
			total += c
		}
	}

	assert.Len(t, published, 200, "every distinct value must be published")
	assert.Equal(t, 200.0, total, "every observation must be accounted for")
}

func TestExport_splitsDatumAcrossRequests(t *testing.T) {
	stub := &stubCloudWatch{}
	srv := newTestServer(stub)
	now := time.Now().UTC().Truncate(time.Minute)

	total := maxMetrics + 1
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("id_%d", i)
		srv.metrics[id] = metricInfo{name: fmt.Sprintf("metric_%d", i), unit: "Count"}
		srv.observations[id] = map[time.Time][]float64{now: {1}}
	}

	srv.doExport(context.Background(), now)

	require.Len(t, stub.requests, 2)

	published := 0
	for _, data := range stub.requests {
		assert.LessOrEqual(t, len(data), maxMetrics)
		published += len(data)
	}

	assert.Equal(t, total, published)
}

func TestGetCounts(t *testing.T) {
	tests := []struct {
		name      string
		input     []float64
		wantPairs map[float64]float64
	}{
		{
			name:      "empty",
			input:     nil,
			wantPairs: map[float64]float64{},
		},
		{
			name:      "single value",
			input:     []float64{1.0},
			wantPairs: map[float64]float64{1.0: 1.0},
		},
		{
			name:      "duplicates",
			input:     []float64{1.0, 1.0, 2.0, 3.0, 3.0, 3.0},
			wantPairs: map[float64]float64{1.0: 2.0, 2.0: 1.0, 3.0: 3.0},
		},
		{
			name:      "all same",
			input:     []float64{5.0, 5.0, 5.0},
			wantPairs: map[float64]float64{5.0: 3.0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			counts, values := getCounts(tc.input)
			require.Equal(t, len(counts), len(values), "counts and values slices must have equal length")

			got := make(map[float64]float64, len(values))
			for i, v := range values {
				got[v] = counts[i]
			}
			assert.Equal(t, tc.wantPairs, got)
		})
	}
}

func TestMetricIdentifier_isDeterministic(t *testing.T) {
	srv := &awsCloudWatchServer{}
	m := metricInfo{
		name:   "requests",
		unit:   "Count",
		labels: []Label{{"env", "prod"}, {"region", "us-east-1"}},
	}
	assert.Equal(t, srv.metricIdentifier(m), srv.metricIdentifier(m))
}

func TestMetricIdentifier_labelOrderIndependent(t *testing.T) {
	srv := &awsCloudWatchServer{}
	m1 := metricInfo{name: "requests", labels: []Label{{"env", "prod"}, {"region", "us-east-1"}}}
	m2 := metricInfo{name: "requests", labels: []Label{{"region", "us-east-1"}, {"env", "prod"}}}
	assert.Equal(t, srv.metricIdentifier(m1), srv.metricIdentifier(m2))
}

func TestMetricIdentifier_differentNamesDifferentIDs(t *testing.T) {
	srv := &awsCloudWatchServer{}
	labels := []Label{{"env", "prod"}}
	id1 := srv.metricIdentifier(metricInfo{name: "requests", labels: labels})
	id2 := srv.metricIdentifier(metricInfo{name: "latency", labels: labels})
	assert.NotEqual(t, id1, id2)
}
