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
	"io"
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
func (adm *AdminClient) ObjectInfo(ctx context.Context, objOpts ObjectInfoOptions) (files map[string]*ObjectInspectInfo, err error) {
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

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	files = make(map[string]*ObjectInspectInfo)
	err = json.Unmarshal(bb, &files)
	if err != nil {
		return nil, err
	}

	return
}

// ObjectInspectPart is returned from minio when calling ObjectInfo
// It contains basic information on a part and it's xl.meta content.
type ObjectInspectPart struct {
	Part            int
	Pool            int
	Host            string
	Set             int
	Drive           string
	Filename        string
	XLVersion       xlMetaV2Version
	XLVersionHeader xlMetaV2VersionHeader
	Stats           StatInfo
	Errors          []string
}

// ObjectInspectInfo is returned from minio when calling ObjectInfo
type ObjectInspectInfo struct {
	Name  string
	Parts []*ObjectInspectPart
}

type xlMetaV2VersionHeader struct {
	VersionID [16]byte
	ModTime   int64
	Signature [4]byte
	Type      int
	Flags     uint8
	EcN, EcM  uint8
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

type xlMetaV2Version struct {
	Type             int                   `json:"Type"`
	ObjectV2         *xlMetaV2Object       `json:"V2Obj,omitempty"`
	DeleteMarker     *xlMetaV2DeleteMarker `json:"DelObj,omitempty"`
	WrittenByVersion uint64                `msg:"v"`
}

type xlMetaV2DeleteMarker struct {
	VersionID [16]byte          `json:"ID"`
	ModTime   int64             `json:"MTime"`
	MetaSys   map[string][]byte `json:"MetaSys,omitempty"`
}

type xlMetaV2Object struct {
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
