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
)

func TestNewServer_noopType_returnsNoOp(t *testing.T) {
	srv, err := NewServer(NoOp, "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := srv.(*noOpServer); !ok {
		t.Errorf("want *noOpServer, got %T", srv)
	}
}

func TestNewServer_unknownType_defaultsToNoOp(t *testing.T) {
	srv, err := NewServer(ServerType("invalid"), "ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := srv.(*noOpServer); !ok {
		t.Errorf("want *noOpServer for unknown type, got %T", srv)
	}
}

func TestNewServer_prometheus_returnsPrometheusServer(t *testing.T) {
	srv, err := NewServer(Prometheus, "ns", WithPort(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := srv.(*prometheusServer); !ok {
		t.Errorf("want *prometheusServer, got %T", srv)
	}
}

func TestNewServer_prometheusPortConflict_returnsError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	_, err = NewServer(Prometheus, "ns", WithPort(port))
	if err == nil {
		t.Error("want error on occupied port, got nil")
	}
}
