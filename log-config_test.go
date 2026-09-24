//
// Copyright (c) 2015-2025 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildInternalRecorderKV(t *testing.T) {
	cases := []struct {
		name string
		cfg  InternalRecorder
		sub  string
		want string
	}{
		{
			name: "enable only",
			cfg:  InternalRecorder{Enable: LogField{Value: "on"}},
			sub:  LogErrorInternalSubSys,
			want: LogErrorInternalSubSys + " enable=on",
		},
		{
			name: "every duration",
			cfg: InternalRecorder{
				Enable:              LogField{Value: "on"},
				Retention:           LogField{Value: "2160h"},
				MaintenanceInterval: LogField{Value: "24h"},
				OrphanGracePeriod:   LogField{Value: "12h"},
			},
			sub:  LogAuditInternalSubSys,
			want: LogAuditInternalSubSys + " enable=on retention=2160h maintenance_interval=24h orphan_grace_period=12h",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildInternalRecorderKV(tc.cfg, tc.sub); got != tc.want {
				t.Fatalf("buildInternalRecorderKV = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseInternalRecorder(t *testing.T) {
	var sc SubsysConfig
	for _, kv := range []ConfigKV{
		{Key: "enable", Value: "on"},
		{Key: "retention", Value: "720h"},
		{Key: "maintenance_interval", Value: "72h"},
		{Key: "orphan_grace_period", Value: "48h"},
		{Key: "drive_limit", Value: "1Gi"},
	} {
		sc.AddConfigKV(kv)
	}
	got := parseInternalRecorder(sc, Help{})
	want := [4]string{"on", "720h", "72h", "48h"}
	if have := [4]string{got.Enable.Value, got.Retention.Value, got.MaintenanceInterval.Value, got.OrphanGracePeriod.Value}; have != want {
		t.Fatalf("parseInternalRecorder = %v, want %v", have, want)
	}
}

func TestLogRecorderConfigYAMLRoundTrip(t *testing.T) {
	in := InternalRecorder{
		Enable:              LogField{Value: "on"},
		Retention:           LogField{Value: "2160h"},
		MaintenanceInterval: LogField{Value: "24h"},
		OrphanGracePeriod:   LogField{Value: "12h"},
	}
	for name, doc := range map[string]string{
		"api":   LogRecorderAPIConfig{Internal: in}.YAML(),
		"error": LogRecorderErrorConfig{Internal: in}.YAML(),
		"audit": LogRecorderAuditConfig{Internal: in}.YAML(),
	} {
		t.Run(name, func(t *testing.T) {
			for _, key := range []string{"retention:", "maintenanceInterval:", "orphanGracePeriod:"} {
				if !strings.Contains(doc, key) {
					t.Fatalf("YAML lacks %s:\n%s", key, doc)
				}
			}
			var out LogRecorderErrorConfig
			if err := yaml.Unmarshal([]byte(doc), &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if out.Internal.Retention.Value != "2160h" || out.Internal.MaintenanceInterval.Value != "24h" || out.Internal.OrphanGracePeriod.Value != "12h" {
				t.Fatalf("round trip lost durations: %+v", out.Internal)
			}
		})
	}
}
