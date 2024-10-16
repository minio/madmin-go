// Copyright (c) 2015-2022 MinIO, Inc.
//
// # This file is part of MinIO Object Storage stack
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

// Package jobs constists of all structs related to  batch job requests and related functionality.
package jobs

import (
	"context"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/pkg/v3/xtime"
)

// BatchJobRequest to start batch job
type BatchJobRequest struct {
	ID        string               `yaml:"-" json:"name"`
	User      string               `yaml:"-" json:"user"`
	Started   time.Time            `yaml:"-" json:"started"`
	Replicate *BatchJobReplicateV1 `yaml:"replicate" json:"replicate"`
	KeyRotate *BatchJobKeyRotateV1 `yaml:"keyrotate" json:"keyrotate"`
	Expire    *BatchJobExpire      `yaml:"expire" json:"expire"`
	Ctx       context.Context      `msg:"-"`
}

// BatchJobReplicateV1 v1 of batch job replication
type BatchJobReplicateV1 struct {
	APIVersion string                  `yaml:"apiVersion" json:"apiVersion"`
	Flags      BatchJobReplicateFlags  `yaml:"flags" json:"flags"`
	Target     BatchJobReplicateTarget `yaml:"target" json:"target"`
	Source     BatchJobReplicateSource `yaml:"source" json:"source"`

	Clnt *miniogo.Core `msg:"-"`
}

// BatchJobReplicateFlags various configurations for replication job definition currently includes
type BatchJobReplicateFlags struct {
	Filter BatchReplicateFilter `yaml:"filter" json:"filter"`
	Notify BatchJobNotification `yaml:"notify" json:"notify"`
	Retry  BatchJobRetry        `yaml:"retry" json:"retry"`
}

// BatchReplicateFilter holds all the filters currently supported for batch replication
type BatchReplicateFilter struct {
	NewerThan     xtime.Duration `yaml:"newerThan,omitempty" json:"newerThan"`
	OlderThan     xtime.Duration `yaml:"olderThan,omitempty" json:"olderThan"`
	CreatedAfter  time.Time      `yaml:"createdAfter,omitempty" json:"createdAfter"`
	CreatedBefore time.Time      `yaml:"createdBefore,omitempty" json:"createdBefore"`
	Tags          []BatchJobKV   `yaml:"tags,omitempty" json:"tags"`
	Metadata      []BatchJobKV   `yaml:"metadata,omitempty" json:"metadata"`
}

// BatchJobKV is a key-value data type which supports wildcard matching
type BatchJobKV struct {
	Line, Col int
	Key       string `yaml:"key" json:"key"`
	Value     string `yaml:"value" json:"value"`
}

// BatchJobNotification stores notification endpoint and token information.
// Used by batch jobs to notify of their status.
type BatchJobNotification struct {
	Line, Col int
	Endpoint  string `yaml:"endpoint" json:"endpoint"`
	Token     string `yaml:"token" json:"token"`
}

// BatchJobRetry stores retry configuration used in the event of failures.
type BatchJobRetry struct {
	Line, Col int
	Attempts  int           `yaml:"attempts" json:"attempts"` // number of retry attempts
	Delay     time.Duration `yaml:"delay" json:"delay"`       // delay between each retries
}

// BatchJobReplicateTarget describes target element of the replication job that receives
// the filtered data from source
type BatchJobReplicateTarget struct {
	Type     BatchJobReplicateResourceType `yaml:"type" json:"type"`
	Bucket   string                        `yaml:"bucket" json:"bucket"`
	Prefix   string                        `yaml:"prefix" json:"prefix"`
	Endpoint string                        `yaml:"endpoint" json:"endpoint"`
	Path     string                        `yaml:"path" json:"path"`
	Creds    BatchJobReplicateCredentials  `yaml:"credentials" json:"credentials"`
}

// BatchJobReplicateResourceType defines the type of batch jobs
type BatchJobReplicateResourceType string

// BatchJobReplicateCredentials access credentials for batch replication it may
// be either for target or source.
type BatchJobReplicateCredentials struct {
	AccessKey    string `xml:"AccessKeyId" json:"accessKey,omitempty" yaml:"accessKey"`
	SecretKey    string `xml:"SecretAccessKey" json:"secretKey,omitempty" yaml:"secretKey"`
	SessionToken string `xml:"SessionToken" json:"sessionToken,omitempty" yaml:"sessionToken"`
}

// BatchJobReplicateSource describes source element of the replication job that is
// the source of the data for the target
type BatchJobReplicateSource struct {
	Type     BatchJobReplicateResourceType `yaml:"type" json:"type"`
	Bucket   string                        `yaml:"bucket" json:"bucket"`
	Prefix   BatchJobPrefix                `yaml:"prefix" json:"prefix"`
	Endpoint string                        `yaml:"endpoint" json:"endpoint"`
	Path     string                        `yaml:"path" json:"path"`
	Creds    BatchJobReplicateCredentials  `yaml:"credentials" json:"credentials"`
	Snowball BatchJobSnowball              `yaml:"snowball" json:"snowball"`
}

// BatchJobPrefix - to support prefix field yaml unmarshalling with string or slice of strings
type BatchJobPrefix []string

// BatchJobSnowball describes the snowball feature when replicating objects from a local source to a remote target
type BatchJobSnowball struct {
	Line, Col   int
	Disable     *bool   `yaml:"disable" json:"disable"`
	Batch       *int    `yaml:"batch" json:"batch"`
	InMemory    *bool   `yaml:"inmemory" json:"inmemory"`
	Compress    *bool   `yaml:"compress" json:"compress"`
	SmallerThan *string `yaml:"smallerThan" json:"smallerThan"`
	SkipErrs    *bool   `yaml:"skipErrs" json:"skipErrs"`
}

// BatchJobKeyRotateV1 v1 of batch key rotation job
type BatchJobKeyRotateV1 struct {
	APIVersion string                      `yaml:"apiVersion" json:"apiVersion"`
	Flags      BatchJobKeyRotateFlags      `yaml:"flags" json:"flags"`
	Bucket     string                      `yaml:"bucket" json:"bucket"`
	Prefix     string                      `yaml:"prefix" json:"prefix"`
	Encryption BatchJobKeyRotateEncryption `yaml:"encryption" json:"encryption"`
}

// BatchJobKeyRotateFlags various configurations for replication job definition currently includes
type BatchJobKeyRotateFlags struct {
	Filter BatchKeyRotateFilter `yaml:"filter" json:"filter"`
	Notify BatchJobNotification `yaml:"notify" json:"notify"`
	Retry  BatchJobRetry        `yaml:"retry" json:"retry"`
}

// BatchKeyRotateFilter holds all the filters currently supported for batch replication
type BatchKeyRotateFilter struct {
	NewerThan     time.Duration `yaml:"newerThan,omitempty" json:"newerThan"`
	OlderThan     time.Duration `yaml:"olderThan,omitempty" json:"olderThan"`
	CreatedAfter  time.Time     `yaml:"createdAfter,omitempty" json:"createdAfter"`
	CreatedBefore time.Time     `yaml:"createdBefore,omitempty" json:"createdBefore"`
	Tags          []BatchJobKV  `yaml:"tags,omitempty" json:"tags"`
	Metadata      []BatchJobKV  `yaml:"metadata,omitempty" json:"metadata"`
	KMSKeyID      string        `yaml:"kmskeyid" json:"kmskey"`
}

// BatchJobKeyRotateEncryption defines key rotation encryption options passed
type BatchJobKeyRotateEncryption struct {
	Type       BatchKeyRotationType `yaml:"type" json:"type"`
	Key        string               `yaml:"key" json:"key"`
	Context    string               `yaml:"context" json:"context"`
	KmsContext map[string]string    `msg:"-"`
}

// BatchKeyRotationType defines key rotation type
type BatchKeyRotationType string

// BatchJobExpire represents configuration parameters for a batch expiration
// job typically supplied in yaml form
type BatchJobExpire struct {
	Line, Col       int
	APIVersion      string                 `yaml:"apiVersion" json:"apiVersion"`
	Bucket          string                 `yaml:"bucket" json:"bucket"`
	Prefix          BatchJobPrefix         `yaml:"prefix" json:"prefix"`
	NotificationCfg BatchJobNotification   `yaml:"notify" json:"notify"`
	Retry           BatchJobRetry          `yaml:"retry" json:"retry"`
	Rules           []BatchJobExpireFilter `yaml:"rules" json:"rules"`
}

// BatchJobExpireFilter holds all the filters currently supported for batch replication
type BatchJobExpireFilter struct {
	Line, Col     int
	OlderThan     xtime.Duration      `yaml:"olderThan,omitempty" json:"olderThan"`
	CreatedBefore *time.Time          `yaml:"createdBefore,omitempty" json:"createdBefore"`
	Tags          []BatchJobKV        `yaml:"tags,omitempty" json:"tags"`
	Metadata      []BatchJobKV        `yaml:"metadata,omitempty" json:"metadata"`
	Size          BatchJobSizeFilter  `yaml:"size" json:"size"`
	Type          string              `yaml:"type" json:"type"`
	Name          string              `yaml:"name" json:"name"`
	Purge         BatchJobExpirePurge `yaml:"purge" json:"purge"`
}

// BatchJobSizeFilter supports size based filters - LesserThan and GreaterThan
type BatchJobSizeFilter struct {
	Line, Col  int
	UpperBound BatchJobSize `yaml:"lessThan" json:"lessThan"`
	LowerBound BatchJobSize `yaml:"greaterThan" json:"greaterThan"`
}

// BatchJobExpirePurge type accepts non-negative versions to be retained
type BatchJobExpirePurge struct {
	Line, Col      int
	RetainVersions int `yaml:"retainVersions" json:"retainVersions"`
}

// BatchJobSize supports humanized byte values in yaml files type BatchJobSize uint64
type BatchJobSize int64
