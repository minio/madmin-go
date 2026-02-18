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

//go:generate msgp -d clearomitted -d "timezone utc" -file $GOFILE

// Table maintenance type constants
const (
	MaintenanceTypeIcebergSnapshotManagement = "icebergSnapshotManagement"
	MaintenanceTypeIcebergCompaction         = "icebergCompaction"
)

// MaintenanceStatus represents the status of a table maintenance configuration.
type MaintenanceStatus string

// Table maintenance status constants
const (
	MaintenanceStatusEnabled  MaintenanceStatus = "enabled"
	MaintenanceStatusDisabled MaintenanceStatus = "disabled"
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
}

// TableMaintenanceSettings is a union type containing maintenance settings.
// Only one of the fields should be set at a time based on the maintenance type.
type TableMaintenanceSettings struct {
	IcebergSnapshotManagement *IcebergSnapshotManagementSettings `json:"icebergSnapshotManagement,omitempty"`
	// IcebergCompaction will be added in a future release.
}

// TableMaintenanceConfigurationValue represents a maintenance configuration with status.
type TableMaintenanceConfigurationValue struct {
	// Settings contains the type-specific maintenance settings.
	Settings *TableMaintenanceSettings `json:"settings,omitempty"`
	// Status indicates whether this maintenance type is enabled or disabled.
	Status MaintenanceStatus `json:"status"`
}

// PutTableMaintenanceConfigurationRequest is the request body for PutTableMaintenanceConfiguration.
type PutTableMaintenanceConfigurationRequest struct {
	Value TableMaintenanceConfigurationValue `json:"value"`
}

// GetTableMaintenanceConfigurationResponse is the response for GetTableMaintenanceConfiguration.
type GetTableMaintenanceConfigurationResponse struct {
	// Configuration maps maintenance type names to their configuration values.
	Configuration map[string]TableMaintenanceConfigurationValue `json:"configuration"`
	// TableARN is the ARN of the table.
	TableARN string `json:"tableARN"`
}
