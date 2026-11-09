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
	"time"
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

func TestIcebergRecordExpirationRulesJSON(t *testing.T) {
	settings := IcebergRecordExpirationSettings{
		Column: "time",
		Rules: []IcebergRecordExpirationRule{
			{Filter: json.RawMessage(`{"type":"eq","term":"retention_class","value":"operational"}`), Days: 365},
		},
	}

	got, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"rules":[{"filter":{"type":"eq","term":"retention_class","value":"operational"},"days":365}],"column":"time"}`
	if string(got) != want {
		t.Errorf("settings encoded as\n %s\nwant\n %s", got, want)
	}

	// The server reads an absent rules field as "keep the stored rules", so an
	// empty list must reach the wire for a client to clear them.
	cleared, err := json.Marshal(IcebergRecordExpirationSettings{Rules: []IcebergRecordExpirationRule{}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(cleared) != `{"rules":[]}` {
		t.Errorf("an empty rule list encoded as %s, want {\"rules\":[]}", cleared)
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

// Compaction's commit-race counts are read by name, and a table whose rewrite
// just landed reports zero losses rather than omitting them.
func TestTableMaintenanceJobTypeStatusCommitRacesLost(t *testing.T) {
	lost, total := int64(3), int64(7)
	lostAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{
		Status:                                MaintenanceJobStatusFailed,
		CompactionCommitRacesLost:             &lost,
		TotalCompactionCommitRacesLost:        &total,
		LastCompactionCommitRaceLostTimestamp: &lostAt,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Failed","compactionCommitRacesLost":3,"totalCompactionCommitRacesLost":7,"lastCompactionCommitRaceLostTimestamp":"2026-11-05T10:00:00Z"}`
	if string(got) != want {
		t.Errorf("a losing compaction encoded as\n %s\nwant\n %s", got, want)
	}

	zero := int64(0)
	got, err = json.Marshal(TableMaintenanceJobTypeStatus{
		Status:                         MaintenanceJobStatusSuccessful,
		CompactionCommitRacesLost:      &zero,
		TotalCompactionCommitRacesLost: &total,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantLanded = `{"status":"Successful","compactionCommitRacesLost":0,"totalCompactionCommitRacesLost":7}`
	if string(got) != wantLanded {
		t.Errorf("a landed compaction encoded as\n %s\nwant\n %s", got, wantLanded)
	}

	got, err = json.Marshal(TableMaintenanceJobTypeStatus{Status: MaintenanceJobStatusSuccessful})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantUnset = `{"status":"Successful"}`
	if string(got) != wantUnset {
		t.Errorf("a status without commit-race counts encoded as\n %s\nwant\n %s", got, wantUnset)
	}
}

// A held table reports the hold in place of a run's outcome.
func TestMaintenanceJobStatusHeldValue(t *testing.T) {
	if MaintenanceJobStatusHeld != "Held" {
		t.Errorf("MaintenanceJobStatusHeld is %q, want %q", MaintenanceJobStatusHeld, "Held")
	}
}

// A skip reason is reported only for a run that was refused, so an unset one
// must not reach the wire.
func TestTableMaintenanceJobTypeStatusSkipReason(t *testing.T) {
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{
		Status:     MaintenanceJobStatusFailed,
		SkipReason: MaintenanceSkipReasonNativeEncryption,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Failed","skipReason":"nativeEncryption"}`
	if string(got) != want {
		t.Errorf("a skipped run encoded as\n %s\nwant\n %s", got, want)
	}

	got, err = json.Marshal(TableMaintenanceJobTypeStatus{Status: MaintenanceJobStatusFailed})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const wantUnset = `{"status":"Failed"}`
	if string(got) != wantUnset {
		t.Errorf("a failed run without a skip reason encoded as\n %s\nwant\n %s", got, wantUnset)
	}

	for reason, wire := range map[MaintenanceSkipReason]string{
		MaintenanceSkipReasonUnsupportedWriteFormat: "unsupportedWriteFormat",
		MaintenanceSkipReasonUnsupportedCodec:       "unsupportedCodec",
		MaintenanceSkipReasonNativeEncryption:       "nativeEncryption",
		MaintenanceSkipReasonFilterUnbound:          "filterUnbound",
		MaintenanceSkipReasonGCDisabled:             "gcDisabled",
		MaintenanceSkipReasonLocationMismatch:       "locationMismatch",
		MaintenanceSkipReasonInvalidClockColumn:     "invalidClockColumn",
		MaintenanceSkipReasonRewriteLimitExceeded:   "rewriteLimitExceeded",
	} {
		if string(reason) != wire {
			t.Errorf("skip reason is %q, want %q", reason, wire)
		}
	}
}

// The compaction backlog is read by name, and a status without it omits it.
func TestTableMaintenanceJobTypeStatusCompactionBacklog(t *testing.T) {
	measured := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	since := measured.Add(-72 * time.Hour)
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{
		Status: MaintenanceJobStatusSuccessful,
		CompactionBacklog: &TableCompactionBacklog{
			MeasuredTimestamp: measured,
			BacklogBytes:      2097152,
			UnplannedFiles:    6,
			DeferredFiles:     24,
			SmallFiles:        &TableCompactionBacklogComponent{Files: 28, ResidualFiles: 28, Ratio: 2.8, GrowthStreak: 3, GrowingSince: &since, Trend: CompactionBacklogGrowing},
			DeleteFiles:       &TableCompactionBacklogComponent{Files: 4, Trend: CompactionBacklogShrinking},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Successful","compactionBacklog":{"measuredTimestamp":"2026-11-06T10:00:00Z","backlogBytes":2097152,"unplannedFiles":6,"deferredFiles":24,` +
		`"smallFiles":{"files":28,"residualFiles":28,"ratio":2.8,"growthStreak":3,"growingSince":"2026-11-03T10:00:00Z","trend":"growing"},` +
		`"deleteFiles":{"files":4,"residualFiles":0,"ratio":0,"trend":"shrinking"}}}`
	if string(got) != want {
		t.Errorf("compaction backlog encoded as\n %s\nwant\n %s", got, want)
	}

	var decoded TableMaintenanceJobTypeStatus
	if err := json.Unmarshal([]byte(want), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d := decoded.CompactionBacklog; d == nil || d.SmallFiles == nil || d.SmallFiles.Trend != CompactionBacklogGrowing || !d.SmallFiles.GrowingSince.Equal(since) || d.Sort != nil {
		t.Errorf("compaction backlog decoded as %+v", decoded.CompactionBacklog)
	}

	got, err = json.Marshal(TableMaintenanceJobTypeStatus{Status: MaintenanceJobStatusSuccessful})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != `{"status":"Successful"}` {
		t.Errorf("a status without compaction backlog encoded as %s", got)
	}
}

// The compaction due timestamp is read by name, and a status without it omits
// it.
func TestTableMaintenanceJobTypeStatusCompactionDue(t *testing.T) {
	due := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	got, err := json.Marshal(TableMaintenanceJobTypeStatus{
		Status:                 MaintenanceJobStatusSuccessful,
		CompactionDueTimestamp: &due,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"status":"Successful","compactionDueTimestamp":"2026-11-07T10:00:00Z"}`
	if string(got) != want {
		t.Errorf("compaction due timestamp encoded as\n %s\nwant\n %s", got, want)
	}

	var decoded TableMaintenanceJobTypeStatus
	if err := json.Unmarshal([]byte(want), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.CompactionDueTimestamp == nil || !decoded.CompactionDueTimestamp.Equal(due) {
		t.Errorf("compaction due timestamp decoded as %v", decoded.CompactionDueTimestamp)
	}
}

func TestAlertTypeTableCompactionBacklogValue(t *testing.T) {
	if AlertTypeTableCompactionBacklog != "table-compaction-backlog" {
		t.Errorf("AlertTypeTableCompactionBacklog = %q", AlertTypeTableCompactionBacklog)
	}
}
