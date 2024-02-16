//
// Copyright (c) 2015-2022 MinIO, Inc.
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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/s3utils"
	"github.com/minio/minio-go/v7/pkg/signer"
	"golang.org/x/net/publicsuffix"
)

// AdminClient implements Amazon S3 compatible methods.
type AdminClient struct {
	///  Standard options.

	// Parsed endpoint url provided by the user.
	endpointURL *url.URL

	// Holds various credential providers.
	credsProvider *credentials.Credentials

	// User supplied.
	appInfo struct {
		appName    string
		appVersion string
	}

	// Indicate whether we are using https or not
	secure bool

	// Needs allocation.
	httpClient *http.Client

	random *rand.Rand

	// Advanced functionality.
	isTraceEnabled bool
	traceOutput    io.Writer
}

// Global constants.
const (
	libraryName    = "madmin-go"
	libraryVersion = "2.0.0"

	libraryAdminURLPrefix = "/minio/admin"
	libraryKMSURLPrefix   = "/minio/kms"
)

// User Agent should always following the below style.
// Please open an issue to discuss any new changes here.
//
//	MinIO (OS; ARCH) LIB/VER APP/VER
const (
	libraryUserAgentPrefix = "MinIO (" + runtime.GOOS + "; " + runtime.GOARCH + ") "
	libraryUserAgent       = libraryUserAgentPrefix + libraryName + "/" + libraryVersion
)

// Options for New method
type Options struct {
	Creds     *credentials.Credentials
	Secure    bool
	Transport http.RoundTripper
	// Add future fields here
}

// New - instantiate minio admin client
// Deprecated: please use NewWithOptions
func New(endpoint string, accessKeyID, secretAccessKey string, secure bool) (*AdminClient, error) {
	creds := credentials.NewStaticV4(accessKeyID, secretAccessKey, "")

	clnt, err := privateNew(endpoint, &Options{Creds: creds, Secure: secure})
	if err != nil {
		return nil, err
	}
	return clnt, nil
}

// NewWithOptions - instantiate minio admin client with options.
func NewWithOptions(endpoint string, opts *Options) (*AdminClient, error) {
	clnt, err := privateNew(endpoint, opts)
	if err != nil {
		return nil, err
	}
	return clnt, nil
}

func privateNew(endpoint string, opts *Options) (*AdminClient, error) {
	// Initialize cookies to preserve server sent cookies if any and replay
	// them upon each request.
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	// construct endpoint.
	endpointURL, err := getEndpointURL(endpoint, opts.Secure)
	if err != nil {
		return nil, err
	}

	clnt := new(AdminClient)

	// Save the credentials.
	clnt.credsProvider = opts.Creds

	// Remember whether we are using https or not
	clnt.secure = opts.Secure

	// Save endpoint URL, user agent for future uses.
	clnt.endpointURL = endpointURL

	tr := opts.Transport
	if tr == nil {
		tr = DefaultTransport(opts.Secure)
	}

	// Instantiate http client and bucket location cache.
	clnt.httpClient = &http.Client{
		Jar:       jar,
		Transport: tr,
	}

	// Add locked pseudo-random number generator.
	clnt.random = rand.New(&lockedRandSource{src: rand.NewSource(time.Now().UTC().UnixNano())})

	// Return.
	return clnt, nil
}

// SetAppInfo - add application details to user agent.
func (adm *AdminClient) SetAppInfo(appName string, appVersion string) {
	// if app name and version is not set, we do not a new user
	// agent.
	if appName != "" && appVersion != "" {
		adm.appInfo.appName = appName
		adm.appInfo.appVersion = appVersion
	}
}

// SetCustomTransport - set new custom transport.
// Deprecated: please use Options{Transport: tr} to provide custom transport.
func (adm *AdminClient) SetCustomTransport(customHTTPTransport http.RoundTripper) {
	// Set this to override default transport
	// ``http.DefaultTransport``.
	//
	// This transport is usually needed for debugging OR to add your
	// own custom TLS certificates on the client transport, for custom
	// CA's and certs which are not part of standard certificate
	// authority follow this example :-
	//
	//   tr := &http.Transport{
	//           TLSClientConfig:    &tls.Config{RootCAs: pool},
	//           DisableCompression: true,
	//   }
	//   api.SetTransport(tr)
	//
	if adm.httpClient != nil {
		adm.httpClient.Transport = customHTTPTransport
	}
}

// TraceOn - enable HTTP tracing.
func (adm *AdminClient) TraceOn(outputStream io.Writer) {
	// if outputStream is nil then default to os.Stdout.
	if outputStream == nil {
		outputStream = os.Stdout
	}
	// Sets a new output stream.
	adm.traceOutput = outputStream

	// Enable tracing.
	adm.isTraceEnabled = true
}

// TraceOff - disable HTTP tracing.
func (adm *AdminClient) TraceOff() {
	// Disable tracing.
	adm.isTraceEnabled = false
}

// requestMetadata - is container for all the values to make a
// request.
type requestData struct {
	customHeaders http.Header
	queryValues   url.Values
	relPath       string // URL path relative to admin API base endpoint
	content       []byte
	contentReader io.Reader
	// endpointOverride overrides target URL with anonymousClient
	endpointOverride *url.URL
	// isKMS replaces URL prefix with /kms
	isKMS bool
}

// Filter out signature value from Authorization header.
func (adm AdminClient) filterSignature(req *http.Request) {
	/// Signature V4 authorization header.

	// Save the original auth.
	origAuth := req.Header.Get("Authorization")
	// Strip out accessKeyID from:
	// Credential=<access-key-id>/<date>/<aws-region>/<aws-service>/aws4_request
	regCred := regexp.MustCompile("Credential=([A-Z0-9]+)/")
	newAuth := regCred.ReplaceAllString(origAuth, "Credential=**REDACTED**/")

	// Strip out 256-bit signature from: Signature=<256-bit signature>
	regSign := regexp.MustCompile("Signature=([[0-9a-f]+)")
	newAuth = regSign.ReplaceAllString(newAuth, "Signature=**REDACTED**")

	// Set a temporary redacted auth
	req.Header.Set("Authorization", newAuth)
}

// dumpHTTP - dump HTTP request and response.
func (adm AdminClient) dumpHTTP(req *http.Request, resp *http.Response) error {
	// Starts http dump.
	_, err := fmt.Fprintln(adm.traceOutput, "---------START-HTTP---------")
	if err != nil {
		return err
	}

	// Filter out Signature field from Authorization header.
	adm.filterSignature(req)

	// Only display request header.
	reqTrace, err := httputil.DumpRequestOut(req, false)
	if err != nil {
		return err
	}

	// Write request to trace output.
	_, err = fmt.Fprint(adm.traceOutput, string(reqTrace))
	if err != nil {
		return err
	}

	// Only display response header.
	var respTrace []byte

	// For errors we make sure to dump response body as well.
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusPartialContent &&
		resp.StatusCode != http.StatusNoContent {
		respTrace, err = httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}
	} else {
		// WORKAROUND for https://github.com/golang/go/issues/13942.
		// httputil.DumpResponse does not print response headers for
		// all successful calls which have response ContentLength set
		// to zero. Keep this workaround until the above bug is fixed.
		if resp.ContentLength == 0 {
			var buffer bytes.Buffer
			if err = resp.Header.Write(&buffer); err != nil {
				return err
			}
			respTrace = buffer.Bytes()
			respTrace = append(respTrace, []byte("\r\n")...)
		} else {
			respTrace, err = httputil.DumpResponse(resp, false)
			if err != nil {
				return err
			}
		}
	}
	// Write response to trace output.
	_, err = fmt.Fprint(adm.traceOutput, strings.TrimSuffix(string(respTrace), "\r\n"))
	if err != nil {
		return err
	}

	// Ends the http dump.
	_, err = fmt.Fprintln(adm.traceOutput, "---------END-HTTP---------")
	return err
}

// do - execute http request.
func (adm AdminClient) do(req *http.Request) (*http.Response, error) {
	resp, err := adm.httpClient.Do(req)
	if err != nil {
		// Handle this specifically for now until future Golang versions fix this issue properly.
		if urlErr, ok := err.(*url.Error); ok {
			if strings.Contains(urlErr.Err.Error(), "EOF") {
				return nil, &url.Error{
					Op:  urlErr.Op,
					URL: urlErr.URL,
					Err: errors.New("Connection closed by foreign host " + urlErr.URL + ". Retry again."),
				}
			}
		}
		return nil, err
	}

	// Response cannot be non-nil, report if its the case.
	if resp == nil {
		msg := "Response is empty. " // + reportIssue
		return nil, ErrInvalidArgument(msg)
	}

	// If trace is enabled, dump http request and response.
	if adm.isTraceEnabled {
		err = adm.dumpHTTP(req, resp)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// List of success status.
var successStatus = []int{
	http.StatusOK,
	http.StatusNoContent,
	http.StatusPartialContent,
}

// RequestData exposing internal data structure requestData
type RequestData struct {
	CustomHeaders http.Header
	QueryValues   url.Values
	RelPath       string // URL path relative to admin API base endpoint
	Content       []byte
}

// ExecuteMethod - similar to internal method executeMethod() useful
// for writing custom requests.
func (adm AdminClient) ExecuteMethod(ctx context.Context, method string, reqData RequestData) (res *http.Response, err error) {
	return adm.executeMethod(ctx, method, requestData{
		customHeaders: reqData.CustomHeaders,
		queryValues:   reqData.QueryValues,
		relPath:       reqData.RelPath,
		content:       reqData.Content,
	})
}

// executeMethod - instantiates a given method, and retries the
// request upon any error up to maxRetries attempts in a binomially
// delayed manner using a standard back off algorithm.
func (adm AdminClient) executeMethod(ctx context.Context, method string, reqData requestData) (res *http.Response, err error) {
	reqRetry := MaxRetry // Indicates how many times we can retry the request
	defer func() {
		if err != nil {
			// close idle connections before returning, upon error.
			adm.httpClient.CloseIdleConnections()
		}
	}()

	// Create cancel context to control 'newRetryTimer' go routine.
	retryCtx, cancel := context.WithCancel(ctx)

	// Indicate to our routine to exit cleanly upon return.
	defer cancel()

	for range adm.newRetryTimer(retryCtx, reqRetry, DefaultRetryUnit, DefaultRetryCap, MaxJitter) {
		// Instantiate a new request.
		var req *http.Request
		req, err = adm.newRequest(ctx, method, reqData)
		if err != nil {
			return nil, err
		}

		// Initiate the request.
		res, err = adm.do(req)
		if err != nil {
			// Give up right away if it is a connection refused problem
			if errors.Is(err, syscall.ECONNREFUSED) {
				return nil, err
			}
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil, err
			}
			// retry all network errors.
			continue
		}

		// For any known successful http status, return quickly.
		for _, httpStatus := range successStatus {
			if httpStatus == res.StatusCode {
				return res, nil
			}
		}

		// Read the body to be saved later.
		errBodyBytes, err := ioutil.ReadAll(res.Body)
		// res.Body should be closed
		closeResponse(res)
		if err != nil {
			return nil, err
		}

		// Save the body.
		errBodySeeker := bytes.NewReader(errBodyBytes)
		res.Body = ioutil.NopCloser(errBodySeeker)

		// For errors verify if its retryable otherwise fail quickly.
		errResponse := ToErrorResponse(httpRespToErrorResponse(res))

		// Save the body back again.
		errBodySeeker.Seek(0, 0) // Seek back to starting point.
		res.Body = ioutil.NopCloser(errBodySeeker)

		// Verify if error response code is retryable.
		if isAdminErrCodeRetryable(errResponse.Code) {
			continue // Retry.
		}

		// Verify if http status code is retryable.
		if isHTTPStatusRetryable(res.StatusCode) {
			continue // Retry.
		}

		break
	}

	// Return an error when retry is canceled or deadlined
	if e := retryCtx.Err(); e != nil {
		return nil, e
	}

	return res, err
}

// set User agent.
func (adm AdminClient) setUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", libraryUserAgent)
	if adm.appInfo.appName != "" && adm.appInfo.appVersion != "" {
		req.Header.Set("User-Agent", libraryUserAgent+" "+adm.appInfo.appName+"/"+adm.appInfo.appVersion)
	}
}

// GetAccessAndSecretKey - retrieves the access and secret keys.
func (adm AdminClient) GetAccessAndSecretKey() (string, string) {
	value, err := adm.credsProvider.Get()
	if err != nil {
		return "", ""
	}
	return value.AccessKeyID, value.SecretAccessKey
}

// GetEndpointURL - returns the endpoint for the admin client.
func (adm AdminClient) GetEndpointURL() *url.URL {
	return adm.endpointURL
}

func (adm AdminClient) getSecretKey() string {
	value, err := adm.credsProvider.Get()
	if err != nil {
		// Return empty, call will fail.
		return ""
	}

	return value.SecretAccessKey
}

// newRequest - instantiate a new HTTP request for a given method.
func (adm AdminClient) newRequest(ctx context.Context, method string, reqData requestData) (req *http.Request, err error) {
	// If no method is supplied default to 'POST'.
	if method == "" {
		method = "POST"
	}

	// Default all requests to ""
	location := ""

	// Construct a new target URL.
	targetURL, err := adm.makeTargetURL(reqData)
	if err != nil {
		return nil, err
	}

	// Initialize a new HTTP request for the method.
	req, err = http.NewRequestWithContext(ctx, method, targetURL.String(), bytes.NewReader(reqData.content))
	if err != nil {
		return nil, err
	}

	value, err := adm.credsProvider.Get()
	if err != nil {
		return nil, err
	}

	var (
		accessKeyID     = value.AccessKeyID
		secretAccessKey = value.SecretAccessKey
		sessionToken    = value.SessionToken
	)

	adm.setUserAgent(req)
	for k, v := range reqData.customHeaders {
		req.Header.Set(k, v[0])
	}
	if length := len(reqData.content); length > 0 {
		req.ContentLength = int64(length)
	}
	sum := sha256.Sum256(reqData.content)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(sum[:]))
	if reqData.contentReader != nil {
		req.Body = ioutil.NopCloser(reqData.contentReader)
	} else {
		req.Body = ioutil.NopCloser(bytes.NewReader(reqData.content))
	}

	req = signer.SignV4(*req, accessKeyID, secretAccessKey, sessionToken, location)
	return req, nil
}

// makeTargetURL make a new target url.
func (adm AdminClient) makeTargetURL(r requestData) (*url.URL, error) {
	host := adm.endpointURL.Host
	scheme := adm.endpointURL.Scheme
	prefix := libraryAdminURLPrefix
	if r.isKMS {
		prefix = libraryKMSURLPrefix
	}
	urlStr := scheme + "://" + host + prefix + r.relPath

	// If there are any query values, add them to the end.
	if len(r.queryValues) > 0 {
		urlStr = urlStr + "?" + s3utils.QueryEncode(r.queryValues)
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil
}
