// Copyright (c) 2020 StackRox Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License

package grpcweb

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/longXboy/go-grpc-http1/internal/sliceutils"

	"google.golang.org/grpc/codes"
)

type jsonWriter struct {
	w http.ResponseWriter

	statusCode int
	header     []byte
	body       *bytes.Buffer
	// List of trailers that were announced via the `Trailer` header at the time headers were written. Also used to keep
	// track of whether headers were already written (in which case this is non-nil, even if it is the empty slice).
	announcedTrailers []string
}

// NewJsonWriter returns a response writer that transparently transcodes an gRPC HTTP/2 response to a gRPC-Web
// response. It can be used as the response writer in the `ServeHTTP` method of a `grpc.Server`.
// The second return value is a finalization function that takes care of sending the data frame with trailers. It
// *needs* to be called before the response handler exits successfully (the returned error is simply any error of the
// underlying response writer passed through).
func NewJsonWriter(w http.ResponseWriter) (http.ResponseWriter, func() error) {
	rw := &jsonWriter{
		w:    w,
		body: bytes.NewBuffer(nil),
	}
	return rw, rw.Finalize
}

// Header returns the HTTP Header of the underlying response writer.
func (w *jsonWriter) Header() http.Header {
	return w.w.Header()
}

// Flush flushes any data not yet written. In contrast to most `http.ResponseWriter` implementations, it does not send
// headers if no data has been written yet.
func (w *jsonWriter) Flush() {
}

// prepareHeadersIfNecessary is called internally on any action that might cause headers to be sent.
func (w *jsonWriter) prepareHeadersIfNecessary() {
	if w.announcedTrailers != nil {
		return
	}

	hdr := w.w.Header()
	w.announcedTrailers = sliceutils.StringClone(hdr["Trailer"])
	// Trailers are sent in a data frame, so don't announce trailers as otherwise downstream proxies might get confused.
	hdr.Del("Trailer")

	hdr.Set("Content-Type", "application/json")

	// Any content length that might be set is no longer accurate because of trailers.
	//hdr.Del("Content-Length")
}

// WriteHeader sends HTTP headers to the client, along with the given status code.
func (w *jsonWriter) WriteHeader(statusCode int) {
	w.prepareHeadersIfNecessary()
	w.statusCode = statusCode
}

// Write writes a chunk of data.
func (w *jsonWriter) Write(buf []byte) (int, error) {
	w.prepareHeadersIfNecessary()

	return w.body.Write(buf)
}

// Finalize sends trailer data in a data frame. It *needs* to be called
func (w *jsonWriter) Finalize() error {
	w.prepareHeadersIfNecessary()
	var body []byte
	if w.body.Len() >= 5 {
		body = w.body.Bytes()[5:]
	} else {
		body = w.body.Bytes()
	}
	w.w.Header().Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
	hdr := w.Header()
	if w.statusCode != 0 {
		w.w.WriteHeader(w.statusCode)
	} else {
		code := new(codes.Code)
		code.UnmarshalJSON([]byte(hdr.Get("Grpc-Status")))
		w.w.WriteHeader(fromGrpcToStatus(*code))
	}
	_, err := w.w.Write(body)
	if err != nil {
		return err
	}
	if flusher, _ := w.w.(http.Flusher); flusher != nil {
		flusher.Flush()
	}
	return nil
}

func fromGrpcToStatus(code codes.Code) (statusCode int) {
	switch code {
	case codes.OK:
		statusCode = 200
	case codes.InvalidArgument:
		statusCode = 400
	case codes.NotFound:
		statusCode = 404
	case codes.PermissionDenied:
		statusCode = 403
	case codes.Unauthenticated:
		statusCode = 401
	case codes.ResourceExhausted:
		statusCode = 429
	case codes.Unimplemented:
		statusCode = 501
	case codes.Aborted:
		statusCode = 444
	case codes.DeadlineExceeded:
		statusCode = 504
	case codes.Unavailable:
		statusCode = 503
	case codes.FailedPrecondition:
		statusCode = 428
	case codes.Unknown:
		statusCode = 500
	default:
		statusCode = 500
	}
	return
}
