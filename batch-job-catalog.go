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

//msgp:tag json
//go:generate msgp -d clearomitted -d "timezone utc" -file $GOFILE

// CatalogDataFile contains information about an output file from a catalog job run.
type CatalogDataFile struct {
	Key         string `json:"key"`
	Size        uint64 `json:"size"`
	MD5Checksum string `json:"MD5Checksum"`
}

// CatalogManifestVersion represents the version of a catalog manifest.
type CatalogManifestVersion string

// Valid values for CatalogManifestVersion.
const (
	// We use AWS S3's manifest file version here as we are following the same
	// format at least initially.
	CatalogManifestVersion1 CatalogManifestVersion = "2016-11-30"
)

// CatalogManifest represents the manifest of a catalog job's result.
type CatalogManifest struct {
	SourceBucket      string                 `json:"sourceBucket"`
	DestinationBucket string                 `json:"destinationBucket"`
	Version           CatalogManifestVersion `json:"version"`
	CreationTimestamp string                 `json:"creationTimestamp"`
	FileFormat        string                 `json:"fileFormat"`
	FileSchema        string                 `json:"fileSchema"`
	Files             []CatalogDataFile      `json:"files"`
}
