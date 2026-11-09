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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package madmin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// The error codes only a recover answers with, beside the codes of
// FilesRefusedBeforeActing.
const (
	// FilesErrExportOpen means the export is open on its node, so a recover
	// has nothing to rebuild: a rebuild would replace a plane in use. HTTP 409.
	FilesErrExportOpen = "ExportOpen"

	// FilesErrJobNotFound means the export's node holds no job with that id.
	// The node forgets a finished job once 64 later jobs have finished, and
	// every job when its gateway restarts; the message says which. Read the
	// export to learn whether it is up. HTTP 404.
	FilesErrJobNotFound = "JobNotFound"
)

// The codes a failed recover job's FilesJobError carries.
const (
	// FilesErrRecoverFailed means the export took the request to rebuild and
	// did not come up. The message carries the rebuild's own reason when it
	// failed, such as a lease another node still holds.
	FilesErrRecoverFailed = "RecoverFailed"

	// FilesErrExportNotOnNode means the node's gateway ended its reload
	// without opening the export: the config in force names no data export
	// with its id, or the export failed before it opened.
	FilesErrExportNotOnNode = "ExportNotOnNode"
)

// FilesJobState is where a job on an export's node is.
type FilesJobState string

const (
	// FilesJobRunning is a job still at work. Read it again.
	FilesJobRunning FilesJobState = "running"

	// FilesJobDone is a job that finished its work.
	FilesJobDone FilesJobState = "done"

	// FilesJobFailed is a job that stopped short. Its Error says why.
	FilesJobFailed FilesJobState = "failed"
)

// The steps of a recover job.
const (
	// FilesRecoverReloading is a recover that asked its node's gateway for a
	// reload, which has not opened the export yet.
	FilesRecoverReloading = "reloading"

	// FilesRecoverRebuilding is a recover whose export opened and is
	// rebuilding from the tier. It takes minutes for a large export.
	FilesRecoverRebuilding = "rebuilding"
)

// FilesJobError is why a job failed: a code, one of the FilesErr* codes its
// operation documents or FilesErrInternalError, and a message for a human.
type FilesJobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// FilesRecoverJob is a recover job: the rebuild of one export's metadata
// plane from the AIStor tier, on the export's node.
type FilesRecoverJob struct {
	// JobID names the job to FilesRecoverJobStatus. It is opaque.
	JobID string `json:"jobID"`

	Name     string `json:"name"`
	ExportID uint64 `json:"exportID"`
	Node     string `json:"node"`

	State FilesJobState `json:"state"`

	// Step is what the job is doing now, or did last: FilesRecoverReloading
	// or FilesRecoverRebuilding. It is empty until the job reports one.
	Step string `json:"step,omitempty"`

	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`

	// Rebuilt is set when the job is done and the export rebuilt from the
	// tier at its request. A done job without it found the export brought up
	// another way, by another open, with nothing rebuilt.
	Rebuilt bool `json:"rebuilt,omitempty"`

	// Error is set when State is FilesJobFailed.
	Error *FilesJobError `json:"error,omitempty"`
}

// RecoverFilesExport asks the export's node to rebuild the export's metadata
// plane from the AIStor tier, and bring the export up. It is the step after
// the drive that holds the export's state is replaced: the export then finds
// no local state over a tier that holds its data, and does not open on its
// own, since no state can also mean another node serves it. The rebuild runs
// under the export's lease, so it fails while another node still holds it.
//
// export is a name or a numeric export id, sent as given. generation is the
// export's FilesExport.Generation the recover was decided on; when the export
// has moved on, the call fails with a FilesExportModifiedError. A recover
// changes no setting of the export, so it leaves the generation as it was.
//
// The call returns once the node has started the job, which runs for minutes
// on a large export. Read it with FilesRecoverJobStatus until its State is no
// longer FilesJobRunning.
//
// An export open on its node is refused with FilesErrExportOpen. A node that
// does not answer fails the call with FilesErrNodeUnreachable; unlike a quota,
// access or remove write, a recover is not recorded, so nothing starts when
// the node answers again. A node whose gateway predates the operation fails it
// with FilesErrNotImplemented.
//
// A request while a recover job runs on the export answers that job, so a
// request whose outcome is unknown is repeated, as for SetFilesExportQuota.
// A repeat that meets FilesErrExportOpen may follow an attempt whose job has
// already brought the export up; read the export to tell.
func (adm *AdminClient) RecoverFilesExport(ctx context.Context, export string, generation FilesGeneration) (FilesRecoverJob, error) {
	reqData, err := filesExportWriteRequest(export, "/recover", generation)
	if err != nil {
		return FilesRecoverJob{}, err
	}

	var job FilesRecoverJob
	err = adm.filesWrite(ctx, filesWriteRequest{
		method: http.MethodPost, reqData: reqData, out: &job,
		repeatUnknown: true,
	})
	return job, err
}

// FilesRecoverJobStatus reads one recover job of export, as RecoverFilesExport
// started it. export is a name or a numeric export id, sent as given. A job
// its node no longer holds is FilesErrJobNotFound.
func (adm *AdminClient) FilesRecoverJobStatus(ctx context.Context, export, jobID string) (FilesRecoverJob, error) {
	switch jobID {
	case "":
		return FilesRecoverJob{}, errors.New("a job id is required")
	case ".", "..":
		return FilesRecoverJob{}, fmt.Errorf("%q is not a job id", jobID)
	}
	relPath, err := filesExportPath(export, "/recover/"+url.PathEscape(jobID))
	if err != nil {
		return FilesRecoverJob{}, err
	}

	var job FilesRecoverJob
	err = adm.filesJSON(ctx, requestData{relPath: relPath}, &job)
	return job, err
}
