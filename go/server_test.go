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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_noopType_returnsNoOp(t *testing.T) {
	srv, err := NewServer(NoOp, "ns")
	require.NoError(t, err)
	assert.IsType(t, &noOpServer{}, srv)
}

func TestNewServer_unknownType_defaultsToNoOp(t *testing.T) {
	srv, err := NewServer(ServerType("invalid"), "ns")
	require.NoError(t, err)
	assert.IsType(t, &noOpServer{}, srv)
}

func TestNewServer_prometheus_returnsPrometheusServer(t *testing.T) {
	srv, err := NewServer(Prometheus, "ns", WithPort(0))
	require.NoError(t, err)
	assert.IsType(t, &prometheusServer{}, srv)
}

func TestNewServer_prometheusPortConflict_returnsError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	_, err = NewServer(Prometheus, "ns", WithPort(port))
	assert.Error(t, err)
}
