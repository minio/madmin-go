//
// Copyright (c) 2015-2026 MinIO, Inc.
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

package madmin

import (
	"encoding/json"
	"testing"
)

// The wire names here are a contract with the AIStor server and the ac client,
// which encode and decode this type independently. A rename that compiles on
// both sides still breaks the API, so the JSON is asserted literally.
func TestIcebergRecordExpirationSettingsJSON(t *testing.T) {
	days, interval := 30, 1440
	settings := TableMaintenanceSettings{
		IcebergRecordExpiration: &IcebergRecordExpirationSettings{
			Days:     &days,
			Column:   "event_time",
			Interval: &interval,
		},
	}

	got, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"icebergRecordExpiration":{"days":30,"column":"event_time","interval":1440}}`
	if string(got) != want {
		t.Errorf("settings encoded as\n %s\nwant\n %s", got, want)
	}

	var back TableMaintenanceSettings
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.IcebergRecordExpiration == nil {
		t.Fatal("record expiration settings did not survive the round trip")
	}
	if back.IcebergRecordExpiration.Days == nil || *back.IcebergRecordExpiration.Days != days {
		t.Errorf("days is %v, want %d", back.IcebergRecordExpiration.Days, days)
	}
	if back.IcebergRecordExpiration.Column != "event_time" {
		t.Errorf("column is %q, want %q", back.IcebergRecordExpiration.Column, "event_time")
	}
	if back.IcebergSnapshotManagement != nil || back.IcebergCompaction != nil ||
		back.IcebergUnreferencedFileRemoval != nil {
		t.Error("a record-expiration payload populated another maintenance type")
	}
}

// An unset clock column must not reach the wire as "column":"". The server
// rejects an empty column, so encoding one turns a client-side omission into a
// server-side error the user cannot act on.
func TestIcebergRecordExpirationSettingsOmitsUnset(t *testing.T) {
	got, err := json.Marshal(IcebergRecordExpirationSettings{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != "{}" {
		t.Errorf("an empty settings value encoded as %s, want {}", got)
	}
}

// The removal counters are reported by record expiration alone. Every other
// maintenance type shares this struct, so they must vanish when unset rather
// than claiming that compaction deleted zero records.
func TestTableMaintenanceJobTypeStatusRemovalCounters(t *testing.T) {
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{Status: MaintenanceJobStatusSuccessful})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Successful"}`
	if string(got) != want {
		t.Errorf("a status with no counters encoded as\n %s\nwant\n %s", got, want)
	}

	records, files, bytes := int64(512), int64(3), int64(4096)
	got, err = json.Marshal(TableMaintenanceJobTypeStatus{
		Status:             MaintenanceJobStatusSuccessful,
		RecordsDeleted:     &records,
		DataFilesRewritten: &files,
		BytesRemoved:       &bytes,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantFull = `{"status":"Successful","recordsDeleted":512,"dataFilesRewritten":3,"bytesRemoved":4096}`
	if string(got) != wantFull {
		t.Errorf("a reporting status encoded as\n %s\nwant\n %s", got, wantFull)
	}

	// Zero is a real answer -- a run that found nothing eligible -- and must be
	// distinguishable from a type that does not report at all.
	zero := int64(0)
	got, err = json.Marshal(TableMaintenanceJobTypeStatus{
		Status:         MaintenanceJobStatusSuccessful,
		RecordsDeleted: &zero,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantZero = `{"status":"Successful","recordsDeleted":0}`
	if string(got) != wantZero {
		t.Errorf("a zero-record run encoded as\n %s\nwant\n %s", got, wantZero)
	}
}

func TestMaintenanceTypeIcebergRecordExpirationValue(t *testing.T) {
	if MaintenanceTypeIcebergRecordExpiration != "icebergRecordExpiration" {
		t.Errorf("maintenance type is %q, want %q",
			MaintenanceTypeIcebergRecordExpiration, "icebergRecordExpiration")
	}
}

// The cumulative counters are the disposal record a retention period is
// evidenced by, so their wire names are part of the exported contract: a
// client reading them by name breaks silently if one is renamed. Nothing else
// pins them, because the struct's other tests only assert omission.
func TestTableMaintenanceJobTypeStatusCumulativeCounters(t *testing.T) {
	total, bytes, runs := int64(9000), int64(65536), int64(12)
	first := "2026-03-01T00:00:00Z"
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{
		Status:              MaintenanceJobStatusSuccessful,
		TotalRecordsDeleted: &total,
		TotalBytesRemoved:   &bytes,
		TotalRuns:           &runs,
		FirstRunTimestamp:   &first,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Successful","totalRecordsDeleted":9000,` +
		`"totalBytesRemoved":65536,"totalRuns":12,"firstRunTimestamp":"2026-03-01T00:00:00Z"}`
	if string(got) != want {
		t.Errorf("a cumulative status encoded as\n %s\nwant\n %s", got, want)
	}

	// A run that removed nothing still counts as a run, so zero must survive
	// the round trip rather than vanishing with omitempty.
	zero := int64(0)
	got, err = json.Marshal(TableMaintenanceJobTypeStatus{
		Status:              MaintenanceJobStatusSuccessful,
		TotalRecordsDeleted: &zero,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantZero = `{"status":"Successful","totalRecordsDeleted":0}`
	if string(got) != wantZero {
		t.Errorf("a zero total encoded as\n %s\nwant\n %s", got, wantZero)
	}
}

// A held table reports the hold in place of a run's outcome.
func TestMaintenanceJobStatusHeldValue(t *testing.T) {
	if MaintenanceJobStatusHeld != "Held" {
		t.Errorf("MaintenanceJobStatusHeld is %q, want %q", MaintenanceJobStatusHeld, "Held")
	}
}
