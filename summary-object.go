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
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/tinylib/msgp/msgp"
)

//msgp:timezone utc
//go:generate msgp -unexported -file=$GOFILE

// ObjectSummaryOptions provides options for ObjectSummary call.
type ObjectSummaryOptions struct {
	Bucket string
	Object string
	Bitrot bool
}

// ObjectSummary calls minio to search for all files and parts
// related to the given object, across all disks.
func (adm *AdminClient) ObjectSummary(ctx context.Context, objOpts ObjectSummaryOptions) (objectSummary *ObjectSummary, err error) {
	form := make(url.Values)
	if objOpts.Bucket == "" {
		return nil, errors.New("no bucket speficied")
	}
	if objOpts.Object == "" {
		return nil, errors.New("no object speficied")
	}

	form.Add("bucket", objOpts.Bucket)
	form.Add("object", objOpts.Object)
	if objOpts.Bitrot {
		form.Add("bitrot", "true")
	}

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/object-summary",
			queryValues: form,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	objectSummary = new(ObjectSummary)
	err = msgp.Decode(resp.Body, objectSummary)
	if err != nil {
		return nil, err
	}

	return
}

// ObjectMetaSummary is returned from minio when calling ObjectSummary
// This struct gives specific information about xl.meta files
// belonging to the object being inspected by the ObjectSummary API.
type ObjectMetaSummary struct {
	Filename       string
	Host           string
	Drive          string
	Size           int64
	Errors         []string
	IsDeleteMarker bool
	ModTime        int64
	Signature      [4]byte
	Pool           int
	Set            int
}

// ObjectPartSummary is returned from minio when calling ObjectSummary.
// This struct gives specific information about each part of the object
// being inspected by the ObjectSummary API.
type ObjectPartSummary struct {
	Part     int
	Pool     int
	Host     string
	Set      int
	Drive    string
	Filename string
	Size     int64
}

// ObjectUnknownSummary is returned from minio when calling ObjectSummary.
// This struct contains information about files that are not part of any object structure.
type ObjectUnknownSummary struct {
	Pool     int
	Host     string
	Set      int
	Drive    string
	Filename string
	Size     int64
	Dir      bool
	Err      string
}

// ObjectVersionSummary is returned from minio when calling ObjectSummary.
// This struct contains information on a single version.
type ObjectVersionSummary struct {
	DeleteMarker bool
	InLined      bool
	Size         int64
	VersionID    string
	Checksum     []byte
	ModTime      time.Time
	DataDir      string
	DataBlocks   int
	ParityBlocks int
}

// ObjectBitrotSummary is returned from minio when calling ObjectSummary.
// This struct contains bitrot information for and object on a specific host and drive.
type ObjectBitrotSummary struct {
	Host  string
	Disk  string
	Parts []int
}

// ObjectSummary is returned from minio when calling ObjectSummary.
type ObjectSummary struct {
	Name   string
	Errors []string
	// DataDir represents the directory on disk created using
	// the version ID's or a random uuid if the object is not
	// versioned.
	DataDir      string
	Version      string
	PartNumbers  []int
	ErasureDist  []uint8
	ParityCount  int // deprecated
	DeleteMarker bool
	Metas        []*ObjectMetaSummary
	Parts        []*ObjectPartSummary
	Unknown      []*ObjectUnknownSummary
	Versions     []*ObjectVersionSummary
	Bitrot       []*ObjectBitrotSummary
}
