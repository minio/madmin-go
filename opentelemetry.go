package madmin

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"

	"github.com/minio/madmin-go/v3/estream"
)

//go:generate msgp $GOFILE

//msgp:replace TraceType with:uint64

// ServiceTelemetryOpts is a request to add following types to tracing.
type ServiceTelemetryOpts struct {
	// Types to add to tracing.
	Types TraceType `json:"types"`

	// Public cert to encrypt stream.
	PubCert []byte

	// Sample rate to set for this filter.
	// If <=0 or >=1 no sampling will be performed
	// and all hits will be traced.
	SampleRate float64 `json:"sampleRate"`

	// Disable sampling and only do tracing when a trace id is set on incoming request.
	ParentOnly bool `json:"parentOnly"`

	// Tag adds a `custom.tag` field to all traces triggered by this.
	TagKV map[string]string `json:"tags"`

	// On incoming HTTP types, apply regex to values to enable tracing.
	HTTPFilter struct {
		Func      string            `json:"funcFilter"`
		UserAgent string            `json:"userAgent"`
		Header    map[string]string `json:"header"`
	} `json:"httpFilter"`
}

//msgp:ignore ServiceTelemetry

// ServiceTelemetry holds http telemetry spans, serialized and compressed.
type ServiceTelemetry struct {
	SpanMZ []byte // Serialized and Compressed spans.
	Err    error  // Any error that occured
}

// ServiceTelemetryStream - gets raw stream for service telemetry.
func (adm AdminClient) ServiceTelemetryStream(ctx context.Context, opts ServiceTelemetryOpts) (io.ReadCloser, error) {
	bopts, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}
	reqData := requestData{
		relPath: adminAPIPrefix + "/telemetry",
		content: bopts,
	}
	// Execute GET to call trace handler
	resp, err := adm.executeMethod(ctx, http.MethodPost, reqData)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		closeResponse(resp)
		return nil, httpRespToErrorResponse(resp)
	}

	return resp.Body, nil
}

// ServiceTelemetry - perform trace request and return individual packages.
// If options contains a public key the private key must be provided.
// If context is canceled the function will return.
func (adm AdminClient) ServiceTelemetry(ctx context.Context, opts ServiceTelemetryOpts, dst chan<- ServiceTelemetry, pk *rsa.PrivateKey) {
	defer close(dst)
	resp, err := adm.ServiceTelemetryStream(ctx, opts)
	if err != nil {
		dst <- ServiceTelemetry{Err: err}
		return
	}
	dec, err := estream.NewReader(resp)
	if err != nil {
		dst <- ServiceTelemetry{Err: err}
		return
	}
	if pk != nil {
		dec.SetPrivateKey(pk)
	}
	for {
		st, err := dec.NextStream()
		if err != nil {
			dst <- ServiceTelemetry{Err: err}
			return
		}
		if ctx.Err() != nil {
			return
		}
		block, err := io.ReadAll(st)
		if err == nil && len(block) == 0 {
			// Ignore 0 sized blocks.
			continue
		}
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case dst <- ServiceTelemetry{SpanMZ: block, Err: err}:
			if err != nil {
				return
			}
		}
	}
}
