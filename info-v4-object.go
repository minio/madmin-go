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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ObjectInfoOptions ...
type ObjectInfoOptions struct {
	Bucket, Object string
}

// ObjectInfo calls minio to search for all files and parts
// related to the given object, across all disks.
func (adm *AdminClient) ObjectInfo(ctx context.Context, objOpts ObjectInfoOptions) (objectInfoResponse *ObjectInfoRespose, err error) {
	form := make(url.Values)
	form.Add("bucket", objOpts.Bucket)
	form.Add("object", objOpts.Object)

	resp, err := adm.executeMethod(ctx,
		http.MethodGet,
		requestData{
			relPath:     fmt.Sprintf(adminAPIPrefixV4 + "/object-info"),
			queryValues: form,
		})
	defer closeResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpRespToErrorResponse(resp)
	}

	objectInfoResponse = new(ObjectInfoRespose)
	objectInfoResponse.files = make(map[string]*ObjectInspectInfo)
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&objectInfoResponse.files)
	if err != nil {
		return nil, err
	}

	return
}

// ObjectInfoRespose ...
// This struct is returned from ObjectInfo
type ObjectInfoRespose struct {
	files map[string]*ObjectInspectInfo
}

// ObjectInspectPart ...
// This struct is returned from minio when calling ObjectInfo.
// It contains basic information on a part and it's xl.meta content.
type ObjectInspectPart struct {
	Part            int
	Pool            int
	Host            string
	Set             int
	Drive           string
	Filename        string
	XLVersion       XLMetaV2Version
	XLVersionHeader XLMetaV2VersionHeader
	Stats           StatInfo
	Errors          []string
}

// ObjectInspectInfo ..
// This struct is returned from minio when calling ObjectInfo.
// It contains the folder name on disk which holds all the parts and xl.meta file
// along with each part.
type ObjectInspectInfo struct {
	Name  string
	Parts []*ObjectInspectPart
}

// StatInfo is returned from minio when calling ObjectInfo
// It contains disk information about the file read.
type StatInfo struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Name    string    `json:"name"`
	Dir     bool      `json:"dir"`
	Mode    uint32    `json:"mode"`
}

// XLMetaV2Version ..
// This struct is returned from minio when calling ObjectInfo.
// It contains xl.meta information about an object part.
type XLMetaV2Version struct {
	Type             int                   `json:"Type"`
	ObjectV2         *XLMetaV2Object       `json:"V2Obj,omitempty"`
	DeleteMarker     *XLMetaV2DeleteMarker `json:"DelObj,omitempty"`
	WrittenByVersion uint64                `json:"v"`
}

// XLMetaV2VersionHeader ..
// This struct is returned by minio when calling ObjectInfo
// It contains basic xl.meta information about an object part
type XLMetaV2VersionHeader struct {
	VersionID [16]byte
	ModTime   int64
	Signature [4]byte
	Type      int
	Flags     uint8
	EcN, EcM  uint8
}

// XLMetaV2DeleteMarker ..
// This struct is returned by minio when calling ObjectInfo
// It is the same as XLMetaV2VersionHeader but for delete markers
type XLMetaV2DeleteMarker struct {
	VersionID [16]byte          `json:"ID"`
	ModTime   int64             `json:"MTime"`
	MetaSys   map[string][]byte `json:"MetaSys,omitempty"`
}

// XLMetaV2Object ...
// This struct is returned form minio when calling ObjectInfo
// It contains detailed information about the object part.
type XLMetaV2Object struct {
	VersionID          [16]byte          `json:"ID"`
	DataDir            [16]byte          `json:"DDir"`
	ErasureAlgorithm   int               `json:"EcAlgo"`
	ErasureM           int               `json:"EcM"`
	ErasureN           int               `json:"EcN" `
	ErasureBlockSize   int64             `json:"EcBSize"`
	ErasureIndex       int               `json:"EcIndex"`
	ErasureDist        []uint8           `json:"EcDist"`
	BitrotChecksumAlgo int               `json:"CSumAlgo"`
	PartNumbers        []int             `json:"PartNums"`
	PartETags          []string          `json:"PartETags"`
	PartSizes          []int64           `json:"PartSizes"`
	PartActualSizes    []int64           `json:"PartASizes,omitempty"`
	PartIndices        [][]byte          `json:"PartIndices,omitempty"`
	Size               int64             `json:"Size"`
	ModTime            int64             `json:"MTime"`
	MetaSys            map[string][]byte `json:"MetaSys,omitempty"`
	MetaUser           map[string]string `json:"MetaUsr,omitempty"`
}
