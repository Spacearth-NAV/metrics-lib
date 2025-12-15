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

type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
}

type noOpLogger struct{}

func (n *noOpLogger) Error(_ string, _ ...any) {
	// intentionally left empty
}

func (n *noOpLogger) Warn(_ string, _ ...any) {
	// intentionally left empty
}

func (n *noOpLogger) Info(_ string, _ ...any) {
	// intentionally left empty
}

func (n *noOpLogger) Debug(_ string, _ ...any) {
	// intentionally left empty
}

var logger Logger = &noOpLogger{}

// SetLogger sets the logger for the metrics package.
// Defaults to a no-op logger.
func SetLogger(l Logger) {
	logger = l
}
