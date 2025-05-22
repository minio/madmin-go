//
// Copyright (c) 2015-2024 MinIO, Inc.
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type (
	// BatchJobType describes batch jobs
	BatchJobType string
	// BatchJobStatusType describes batch job statuses
	BatchJobStatusType string
)

const (
	BatchJobReplicate BatchJobType = "replicate"
	BatchJobKeyRotate BatchJobType = "keyrotate"
	BatchJobExpire    BatchJobType = "expire"
	BatchJobCatalog   BatchJobType = "catalog"
)

const (
	BatchJobStatusCompleted  BatchJobStatusType = "completed"
	BatchJobStatusFailed     BatchJobStatusType = "failed"
	BatchJobStatusInProgress BatchJobStatusType = "in-progress"
	BatchJobStatusUnknown    BatchJobStatusType = "unknown"
)

// SupportedJobTypes supported job types
var SupportedJobTypes = []BatchJobType{
	BatchJobReplicate,
	BatchJobKeyRotate,
	BatchJobExpire,
	// No need to add new types here, they are added directly in server.
}

// BatchJobReplicateTemplate provides a sample template
// for batch replication
const BatchJobReplicateTemplate = `replicate:
  apiVersion: v1
  # source of the objects to be replicated
  source:
    type: TYPE # valid values are "s3" or "minio"
    bucket: BUCKET
    prefix: PREFIX # 'PREFIX' is optional
    # If your source is the 'local' alias specified to 'mc batch start', then the 'endpoint' and 'credentials' fields are optional and can be omitted
    # Either the 'source' or 'remote' *must* be the "local" deployment
    endpoint: "http[s]://HOSTNAME:PORT"
    # path: "on|off|auto" # "on" enables path-style bucket lookup. "off" enables virtual host (DNS)-style bucket lookup. Defaults to "auto"
    credentials:
      accessKey: ACCESS-KEY # Required
      secretKey: SECRET-KEY # Required
    # sessionToken: SESSION-TOKEN # Optional only available when rotating credentials are used
    snowball: # automatically activated if the source is local
      disable: false # optionally turn-off snowball archive transfer
      batch: 100 # upto this many objects per archive
      inmemory: true # indicates if the archive must be staged locally or in-memory
      compress: false # S2/Snappy compressed archive
      smallerThan: 256KiB # create archive for all objects smaller than 256KiB
      skipErrs: false # skips any source side read() errors

  # target where the objects must be replicated
  target:
    type: TYPE # valid values are "s3" or "minio"
    bucket: BUCKET
    prefix: PREFIX # 'PREFIX' is optional
    # If your source is the 'local' alias specified to 'mc batch start', then the 'endpoint' and 'credentials' fields are optional and can be omitted

    # Either the 'source' or 'remote' *must* be the "local" deployment
    endpoint: "http[s]://HOSTNAME:PORT"
    # path: "on|off|auto" # "on" enables path-style bucket lookup. "off" enables virtual host (DNS)-style bucket lookup. Defaults to "auto"
    credentials:
      accessKey: ACCESS-KEY
      secretKey: SECRET-KEY
    # sessionToken: SESSION-TOKEN # Optional only available when rotating credentials are used

  # NOTE: All flags are optional
  # - filtering criteria only applies for all source objects match the criteria
  # - configurable notification endpoints
  # - configurable retries for the job (each retry skips successfully previously replaced objects)
  flags:
    filter:
      newerThan: "7d" # match objects newer than this value (e.g. 7d10h31s)
      olderThan: "7d" # match objects older than this value (e.g. 7d10h31s)
      createdAfter: "date" # match objects created after "date"
      createdBefore: "date" # match objects created before "date"

      ## NOTE: tags are not supported when "source" is remote.
      # tags:
      #   - key: "name"
      #     value: "pick*" # match objects with tag 'name', with all values starting with 'pick'

      # metadata:
      #   - key: "content-type"
      #     value: "image/*" # match objects with 'content-type', with all values starting with 'image/'

    notify:
      endpoint: "https://notify.endpoint" # notification endpoint to receive job status events
      token: "Bearer xxxxx" # optional authentication token for the notification endpoint

    retry:
      attempts: 10 # number of retries for the job before giving up
      delay: "500ms" # least amount of delay between each retry
`

// BatchJobKeyRotateTemplate provides a sample template
// for batch key rotation
const BatchJobKeyRotateTemplate = `keyrotate:
  apiVersion: v1
  bucket: BUCKET
  prefix: PREFIX
  encryption:
    type: sse-s3 # valid values are sse-s3 and sse-kms
    key: <new-kms-key> # valid only for sse-kms
    context: <new-kms-key-context> # valid only for sse-kms

  # optional flags based filtering criteria
  # for all objects
  flags:
    filter:
      newerThan: "7d" # match objects newer than this value (e.g. 7d10h31s)
      olderThan: "7d" # match objects older than this value (e.g. 7d10h31s)
      createdAfter: "date" # match objects created after "date"
      createdBefore: "date" # match objects created before "date"
      tags:
        - key: "name"
          value: "pick*" # match objects with tag 'name', with all values starting with 'pick'
      metadata:
        - key: "content-type"
          value: "image/*" # match objects with 'content-type', with all values starting with 'image/'
      kmskey: "key-id" # match objects with KMS key-id (applicable only for sse-kms)
    notify:
      endpoint: "https://notify.endpoint" # notification endpoint to receive job status events
      token: "Bearer xxxxx" # optional authentication token for the notification endpoint
    retry:
      attempts: 10 # number of retries for the job before giving up
      delay: "500ms" # least amount of delay between each retry
`

// BatchJobExpireTemplate provides a sample template
// for batch expiring objects
const BatchJobExpireTemplate = `expire:
  apiVersion: v1
  bucket: mybucket # Bucket where this job will expire matching objects from
  prefix: myprefix # (Optional) Prefix under which this job will expire objects matching the rules below.
  rules:
    - type: object  # objects with zero ore more older versions
      name: NAME # match object names that satisfy the wildcard expression.
      olderThan: 70h # match objects older than this value
      createdBefore: "2006-01-02T15:04:05.00Z" # match objects created before "date"
      tags:
        - key: name
          value: pick* # match objects with tag 'name', all values starting with 'pick'
      metadata:
        - key: content-type
          value: image/* # match objects with 'content-type', all values starting with 'image/'
      size:
        lessThan: 10MiB # match objects with size less than this value (e.g. 10MiB)
        greaterThan: 1MiB # match objects with size greater than this value (e.g. 1MiB)
      purge:
          # retainVersions: 0 # (default) delete all versions of the object. This option is the fastest.
          # retainVersions: 5 # keep the latest 5 versions of the object.

    - type: deleted # objects with delete marker as their latest version
      name: NAME # match object names that satisfy the wildcard expression.
      olderThan: 10h # match objects older than this value (e.g. 7d10h31s)
      createdBefore: "2006-01-02T15:04:05.00Z" # match objects created before "date"
      purge:
          # retainVersions: 0 # (default) delete all versions of the object. This option is the fastest.
          # retainVersions: 5 # keep the latest 5 versions of the object including delete markers.

  notify:
    endpoint: https://notify.endpoint # notification endpoint to receive job completion status
    token: Bearer xxxxx # optional authentication token for the notification endpoint

  retry:
    attempts: 10 # number of retries for the job before giving up
    delay: 500ms # least amount of delay between each retry
`

// BatchJobResult returned by StartBatchJob
type BatchJobResult struct {
	ID      string             `json:"id"`
	Type    BatchJobType       `json:"type"`
	User    string             `json:"user,omitempty"`
	Started time.Time          `json:"started"`
	Elapsed time.Duration      `json:"elapsed,omitempty"`
	Status  BatchJobStatusType `json:"status"`
}

// StartBatchJob start a new batch job, input job description is in YAML.
func (adm *AdminClient) StartBatchJob(ctx context.Context, job string) (BatchJobResult, error) {
	resp, err := adm.executeMethod(ctx, http.MethodPost,
		requestData{
			relPath: adminAPIPrefix + "/start-job",
			content: []byte(job),
		},
	)
	if err != nil {
		return BatchJobResult{}, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return BatchJobResult{}, httpRespToErrorResponse(resp)
	}

	res := BatchJobResult{}
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&res); err != nil {
		return res, err
	}

	return res, nil
}

// BatchJobStatus contains the last batch job metric
type BatchJobStatus struct {
	LastMetric JobMetric
}

// BatchJobStatus returns the status of the given job.
func (adm *AdminClient) BatchJobStatus(ctx context.Context, jobID string) (BatchJobStatus, error) {
	values := make(url.Values)
	values.Set("jobId", jobID)

	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/status-job",
			queryValues: values,
		},
	)
	if err != nil {
		return BatchJobStatus{}, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return BatchJobStatus{}, httpRespToErrorResponse(resp)
	}

	res := BatchJobStatus{}
	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&res); err != nil {
		return res, err
	}

	return res, nil
}

// DescribeBatchJob - describes a currently running Job.
func (adm *AdminClient) DescribeBatchJob(ctx context.Context, jobID string) (string, error) {
	values := make(url.Values)
	values.Set("jobId", jobID)

	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/describe-job",
			queryValues: values,
		},
	)
	if err != nil {
		return "", err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return "", httpRespToErrorResponse(resp)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// GenerateBatchJobOpts is to be implemented in future.
type GenerateBatchJobOpts struct {
	Type BatchJobType
}

// GenerateBatchJob creates a new job template from standard template
// TODO: allow configuring yaml values
func (adm *AdminClient) GenerateBatchJob(_ context.Context, opts GenerateBatchJobOpts) (string, error) {
	// TODO: allow configuring the template to fill values from GenerateBatchJobOpts
	switch opts.Type {
	case BatchJobReplicate:
		return BatchJobReplicateTemplate, nil
	case BatchJobKeyRotate:
		return BatchJobKeyRotateTemplate, nil
	case BatchJobExpire:
		return BatchJobExpireTemplate, nil
	}
	return "", fmt.Errorf("unknown batch job requested: %s", opts.Type)
}

// GetSupportedBatchJobTypes returns the list of server supported batch job
// types.
func (adm *AdminClient) GetSupportedBatchJobTypes(ctx context.Context) (supportedTypes []BatchJobType, apiUnavailable bool, err error) {
	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath: adminAPIPrefix + "/list-supported-job-types",
		},
	)
	if err != nil {
		return nil, false, err
	}
	defer closeResponse(resp)

	var buf []byte
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusUpgradeRequired:
		apiUnavailable = true
	case http.StatusOK:
		if buf, err = io.ReadAll(resp.Body); err == nil {
			err = json.Unmarshal(buf, &supportedTypes)
		}
	default:
		err = httpRespToErrorResponse(resp)
	}
	return supportedTypes, apiUnavailable, err
}

// GenerateBatchJobV2 creates a new job template by requesting the server. This
// is an EOS only API and returns apiUnavailable=true when API is unsupported.
func (adm *AdminClient) GenerateBatchJobV2(ctx context.Context, opts GenerateBatchJobOpts) (template string, apiUnavailable bool, err error) {
	if opts.Type == "" {
		err = fmt.Errorf("batch job type is required")
		return "", false, err
	}

	values := make(url.Values)
	values.Set("jobType", string(opts.Type))

	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/generate-job",
			queryValues: values,
		},
	)
	if err != nil {
		return "", false, err
	}
	defer closeResponse(resp)

	var buf []byte
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusUpgradeRequired:
		apiUnavailable = true
	case http.StatusOK:
		if buf, err = io.ReadAll(resp.Body); err == nil {
			template = string(buf)
		}
	default:
		err = httpRespToErrorResponse(resp)
	}
	return template, apiUnavailable, err
}

// ListBatchJobsResult contains entries for all current jobs.
type ListBatchJobsResult struct {
	Jobs []BatchJobResult `json:"jobs"`
}

// ListBatchJobsFilter returns list based on following
// filtering params.
type ListBatchJobsFilter struct {
	ByJobType string
}

// ListBatchJobs list all the currently active batch jobs
func (adm *AdminClient) ListBatchJobs(ctx context.Context, fl *ListBatchJobsFilter) (ListBatchJobsResult, error) {
	if fl == nil {
		return ListBatchJobsResult{}, errors.New("ListBatchJobsFilter cannot be nil")
	}

	values := make(url.Values)
	values.Set("jobType", fl.ByJobType)

	resp, err := adm.executeMethod(ctx, http.MethodGet,
		requestData{
			relPath:     adminAPIPrefix + "/list-jobs",
			queryValues: values,
		},
	)
	if err != nil {
		return ListBatchJobsResult{}, err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return ListBatchJobsResult{}, httpRespToErrorResponse(resp)
	}

	d := json.NewDecoder(resp.Body)
	result := ListBatchJobsResult{}
	if err = d.Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}

// CancelBatchJob cancels ongoing batch job.
func (adm *AdminClient) CancelBatchJob(ctx context.Context, jobID string) error {
	values := make(url.Values)
	values.Set("id", jobID)

	resp, err := adm.executeMethod(ctx, http.MethodDelete,
		requestData{
			relPath:     adminAPIPrefix + "/cancel-job",
			queryValues: values,
		},
	)
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusNoContent {
		return httpRespToErrorResponse(resp)
	}
	return nil
}

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
