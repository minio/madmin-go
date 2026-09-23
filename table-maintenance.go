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
//

package madmin

import "encoding/json"

// Table maintenance type constants
const (
	MaintenanceTypeIcebergSnapshotManagement      = "icebergSnapshotManagement"
	MaintenanceTypeIcebergCompaction              = "icebergCompaction"
	MaintenanceTypeIcebergUnreferencedFileRemoval = "icebergUnreferencedFileRemoval"
	MaintenanceTypeIcebergRecordExpiration        = "icebergRecordExpiration"
)

// IcebergSnapshotManagementSettings contains settings for Iceberg snapshot management.
// This configures automatic snapshot expiration based on age and retention policies.
type IcebergSnapshotManagementSettings struct {
	// MaxSnapshotAgeHours specifies the maximum age in hours before snapshots are expired.
	// Must be at least 1 if specified.
	MaxSnapshotAgeHours *int `json:"maxSnapshotAgeHours,omitempty"`
	// MinSnapshotsToKeep specifies the minimum number of snapshots to retain.
	// Must be at least 1 if specified.
	MinSnapshotsToKeep *int `json:"minSnapshotsToKeep,omitempty"`
	// Interval overrides how often this runs, in minutes.
	// Inherits: table -> warehouse -> server if nil.
	// Must be >= 1.
	Interval *int `json:"interval,omitempty"`
}

// IcebergCompactionSettings contains settings for Iceberg table compaction.
type IcebergCompactionSettings struct {
	// TargetFileSizeMB is the target file size in MB for compacted files.
	TargetFileSizeMB *int `json:"targetFileSizeMB,omitempty"`
	// Interval overrides how often this runs, in minutes.
	// Inherits: table -> warehouse -> server if nil.
	// Must be >= 1.
	Interval *int `json:"interval,omitempty"`
	// Filter is an optional Iceberg expression, in the REST spec's JSON filter
	// form, that scopes compaction to the data files it can match; matched files
	// are rewritten in full so no rows are dropped. It is table-scoped only (it
	// names a table's columns/partitions) and is rejected at the warehouse level.
	Filter json.RawMessage `json:"filter,omitempty"`
}

// IcebergUnreferencedFileRemovalSettings contains settings for Iceberg unreferenced file removal.
// Files are eligible to be marked noncurrent when their age (anchored to creation time) exceeds
// UnreferencedDays. Once noncurrent, files are permanently deleted after NoncurrentDays.
type IcebergUnreferencedFileRemovalSettings struct {
	UnreferencedDays *int `json:"unreferencedDays,omitempty"`
	NoncurrentDays   *int `json:"noncurrentDays,omitempty"`
	// Interval overrides how often this runs, in minutes.
	// Inherits: table -> warehouse -> server if nil.
	// Must be >= 1.
	Interval *int `json:"interval,omitempty"`
}

// IcebergRecordExpirationSettings contains settings for Iceberg record expiration:
// a data retention period that removes rows, not files. A run commits a snapshot
// without the rows whose Column value is older than Days; snapshot expiration and
// unreferenced file removal then reclaim the bytes.
//
// Unlike the other maintenance types this one is table-scoped only. It names a
// column of one table's schema, so it is rejected at the warehouse level.
type IcebergRecordExpirationSettings struct {
	// Days is the retention period. A row is eligible once its Column value is
	// more than Days old. Must be >= 1.
	Days *int `json:"days,omitempty"`
	// Column is the clock: a top-level timestamp, timestamptz or date field of
	// the table's current schema. AWS selects this itself because it only
	// expires records on tables whose schema it owns; on user tables it has to
	// be named.
	Column string `json:"column,omitempty"`
	// Interval overrides how often this runs, in minutes.
	// Inherits: table -> warehouse -> server if nil.
	// Must be >= 1.
	Interval *int `json:"interval,omitempty"`
}

// TableMaintenanceSettings is a union type containing maintenance settings.
// Only one of the fields should be set at a time based on the maintenance type.
type TableMaintenanceSettings struct {
	IcebergSnapshotManagement      *IcebergSnapshotManagementSettings      `json:"icebergSnapshotManagement,omitempty"`
	IcebergCompaction              *IcebergCompactionSettings              `json:"icebergCompaction,omitempty"`
	IcebergUnreferencedFileRemoval *IcebergUnreferencedFileRemovalSettings `json:"icebergUnreferencedFileRemoval,omitempty"`
	IcebergRecordExpiration        *IcebergRecordExpirationSettings        `json:"icebergRecordExpiration,omitempty"`
}

// TableMaintenanceConfigurationValue represents a maintenance configuration with status.
type TableMaintenanceConfigurationValue struct {
	Settings *TableMaintenanceSettings `json:"settings,omitempty"`
	Status   MaintenanceStatus         `json:"status"`
	// Override is true if Settings+Status came from the table's own
	// maintenance override, false if inherited from the warehouse default.
	Override bool `json:"override"`
}

// PutTableMaintenanceConfigurationRequest is the request body for PutTableMaintenanceConfiguration.
type PutTableMaintenanceConfigurationRequest struct {
	Value TableMaintenanceConfigurationValue `json:"value"`
}

// GetTableMaintenanceConfigurationResponse is the response for GetTableMaintenanceConfiguration.
type GetTableMaintenanceConfigurationResponse struct {
	Configuration map[string]TableMaintenanceConfigurationValue `json:"configuration"`
	TableARN      string                                        `json:"tableARN"`
}

// MaintenanceJobStatus represents the outcome of a maintenance job's last run.
type MaintenanceJobStatus string

const (
	MaintenanceJobStatusSuccessful MaintenanceJobStatus = "Successful"
	MaintenanceJobStatusFailed     MaintenanceJobStatus = "Failed"
	MaintenanceJobStatusDisabled   MaintenanceJobStatus = "Disabled"
	MaintenanceJobStatusNotYetRun  MaintenanceJobStatus = "Not_Yet_Run"
	// MaintenanceJobStatusHeld reports a table whose maintenance is suspended
	// by a legal hold. It describes the state of the table rather than the
	// outcome of a run, so it appears in place of one on a table that has
	// never run as well as on one that has.
	MaintenanceJobStatusHeld MaintenanceJobStatus = "Held"
)

// TableMaintenanceJobTypeStatus is the per-type status entry in GetTableMaintenanceJobStatusResponse.
type TableMaintenanceJobTypeStatus struct {
	Status           MaintenanceJobStatus `json:"status"`
	LastRunTimestamp *string              `json:"lastRunTimestamp,omitempty"`
	FailureMessage   string               `json:"failureMessage,omitempty"`

	// RecordsDeleted, DataFilesRewritten and BytesRemoved report what the last
	// run removed. Only record expiration reports them, because it is the only
	// maintenance type that ends the life of data rather than of files or
	// history, and a retention period has to be auditable to be evidence of
	// disposal.
	RecordsDeleted     *int64 `json:"recordsDeleted,omitempty"`
	DataFilesRewritten *int64 `json:"dataFilesRewritten,omitempty"`
	BytesRemoved       *int64 `json:"bytesRemoved,omitempty"`

	// TotalRecordsDeleted, TotalBytesRemoved, TotalRuns and FirstRunTimestamp
	// accumulate across runs, where the three above describe only the last
	// one.
	//
	// The last run cannot evidence a retention period: the run after one that
	// deleted rows overwrites its counts with zeros, so a table that disposed
	// of millions of rows reports 0 for the rest of the interval. These are
	// monotonic, so "what has this retention period removed" always has an
	// answer.
	TotalRecordsDeleted *int64  `json:"totalRecordsDeleted,omitempty"`
	TotalBytesRemoved   *int64  `json:"totalBytesRemoved,omitempty"`
	TotalRuns           *int64  `json:"totalRuns,omitempty"`
	FirstRunTimestamp   *string `json:"firstRunTimestamp,omitempty"`
}

// GetTableMaintenanceJobStatusResponse is the response for GetTableMaintenanceJobStatus.
type GetTableMaintenanceJobStatusResponse struct {
	TableARN string                                   `json:"tableARN"`
	Status   map[string]TableMaintenanceJobTypeStatus `json:"status"`
}

// WarehouseMaintenanceConfigurationValue represents a maintenance configuration with status
// for a warehouse. It is structurally identical to TableMaintenanceConfigurationValue but
// scoped to a warehouse rather than an individual table.
type WarehouseMaintenanceConfigurationValue struct {
	Settings *TableMaintenanceSettings `json:"settings,omitempty"`
	Status   MaintenanceStatus         `json:"status"`
}

// PutWarehouseMaintenanceConfigurationRequest is the request body for PutWarehouseMaintenanceConfiguration.
type PutWarehouseMaintenanceConfigurationRequest struct {
	Value WarehouseMaintenanceConfigurationValue `json:"value"`
}

// GetWarehouseMaintenanceConfigurationResponse is the response for GetWarehouseMaintenanceConfiguration.
type GetWarehouseMaintenanceConfigurationResponse struct {
	Configuration map[string]WarehouseMaintenanceConfigurationValue `json:"configuration"`
	WarehouseARN  string                                            `json:"warehouseARN"`
}
