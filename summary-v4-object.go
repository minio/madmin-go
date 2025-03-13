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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/tinylib/msgp/msgp"
)

//go:generate msgp -unexported -file=$GOFILE

// ObjectSummaryOptions ...
type ObjectSummaryOptions struct {
	Bucket, Object string
}

// ObjectSummary calls minio to search for all files and parts
// related to the given object, across all disks.
func (adm *AdminClient) ObjectSummary(ctx context.Context, objOpts ObjectSummaryOptions) (objectSummary *ObjectSummary, err error) {
	form := make(url.Values)
	if objOpts.Bucket == "" {
		return nil, errors.New("You must specify a bucket")
	}
	if objOpts.Object == "" {
		return nil, errors.New("You must specify an object")
	}

	form.Add("bucket", objOpts.Bucket)
	form.Add("object", objOpts.Object)

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     fmt.Sprintf(adminAPIPrefix + "/object-summary"),
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

// ObjectMetaSummary ...
// This struct is returned from minio when calling ObjectSummary
type ObjectMetaSummary struct {
	Filename       string
	Host           string
	Drive          string
	Size           int64
	Errors         []string
	IsDeleteMarker bool
	ModTime        int64
	Signature      [4]byte
}

// ObjectSummaryPart ...
// This struct is returned from minio when calling ObjectSummary.
// It contains basic information on a part and it's xl.meta content.
type ObjectPartSummary struct {
	Part     int
	Pool     int
	Host     string
	Set      int
	Drive    string
	Filename string
	Size     int64
}

// ObjectSummaryInfo ..
// This struct is returned from minio when calling ObjectSummary.
type ObjectSummary struct {
	// Name is the object directory name as seen on disk.
	// More specifically the directory that contains the xl.meta file.
	Name        string
	Errors      []string
	DataDir     string
	IsInline    bool
	PartNumbers []int
	ErasureDist []uint8
	Metas       []*ObjectMetaSummary
	Parts       []*ObjectPartSummary
}
