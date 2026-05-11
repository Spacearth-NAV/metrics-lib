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
	"testing"
)

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
			if len(counts) != len(values) {
				t.Fatalf("len mismatch: counts=%d values=%d", len(counts), len(values))
			}
			got := make(map[float64]float64, len(values))
			for i, v := range values {
				got[v] = counts[i]
			}
			if len(got) != len(tc.wantPairs) {
				t.Errorf("want %d pairs, got %d", len(tc.wantPairs), len(got))
			}
			for v, wantCount := range tc.wantPairs {
				if got[v] != wantCount {
					t.Errorf("value %f: want count %f, got %f", v, wantCount, got[v])
				}
			}
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
	if id1, id2 := srv.metricIdentifier(m), srv.metricIdentifier(m); id1 != id2 {
		t.Errorf("same input → different IDs: %q vs %q", id1, id2)
	}
}

func TestMetricIdentifier_labelOrderIndependent(t *testing.T) {
	srv := &awsCloudWatchServer{}
	m1 := metricInfo{name: "requests", labels: []Label{{"env", "prod"}, {"region", "us-east-1"}}}
	m2 := metricInfo{name: "requests", labels: []Label{{"region", "us-east-1"}, {"env", "prod"}}}
	if srv.metricIdentifier(m1) != srv.metricIdentifier(m2) {
		t.Error("same labels in different order → different IDs")
	}
}

func TestMetricIdentifier_differentNamesDifferentIDs(t *testing.T) {
	srv := &awsCloudWatchServer{}
	labels := []Label{{"env", "prod"}}
	id1 := srv.metricIdentifier(metricInfo{name: "requests", labels: labels})
	id2 := srv.metricIdentifier(metricInfo{name: "latency", labels: labels})
	if id1 == id2 {
		t.Error("different metric names → same ID")
	}
}
