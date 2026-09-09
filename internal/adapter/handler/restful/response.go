package restful

import (
	"encoding/json"
	"errors"
	"net/http"
)

// noWritten marks a responseWriter whose status line has not been flushed yet.
const noWritten = -1

// contentTypeJSON is the content type the handlers answer with.
const contentTypeJSON = "application/json; charset=utf-8"

// responseWriter defers the status code until the first body write. Handlers
// select a status first and render afterwards, so the status has to be
// buffered until there are bytes to send; that also lets a middleware still
// override the status an inner layer picked, as long as nothing was flushed.
type responseWriter struct {
	http.ResponseWriter
	size   int
	status int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, size: noWritten, status: http.StatusOK}
}

// writer returns the deferred writer the router installs for every request.
// The fallback keeps the render helpers usable with a bare http.ResponseWriter.
func writer(w http.ResponseWriter) *responseWriter {
	if rw, ok := w.(*responseWriter); ok {
		return rw
	}
	return newResponseWriter(w)
}

// WriteHeader records the status instead of sending it. Once the response has
// been flushed the status is fixed and later calls are ignored.
func (w *responseWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.written() {
			return
		}
		w.status = code
	}
}

// WriteHeaderNow flushes the recorded status if that has not happened yet.
func (w *responseWriter) WriteHeaderNow() {
	if !w.written() {
		w.size = 0
		w.ResponseWriter.WriteHeader(w.status)
	}
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

// Status reports the status that was or will be sent.
func (w *responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) written() bool {
	return w.size != noWritten
}

// writeJSON renders obj as JSON under the given status code. Content-Type is
// only defaulted when the handler did not pick one, and the body is produced
// with json.Marshal, which HTML-escapes and appends no trailing newline.
func writeJSON(w http.ResponseWriter, code int, obj any) {
	rw := writer(w)
	rw.WriteHeader(code)
	header := rw.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{contentTypeJSON}
	}
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	if _, err := rw.Write(data); err != nil {
		panic(err)
	}
}

// abortWithStatus flushes a bare status line with no body.
func abortWithStatus(w http.ResponseWriter, code int) {
	rw := writer(w)
	rw.WriteHeader(code)
	rw.WriteHeaderNow()
}

var errInvalidRequest = errors.New("invalid request")

// bindJSON decodes the request body into obj. A decode failure flushes 400
// immediately and still returns the error, so the caller reports it too. The
// request payload structs carry no validation tags, so decoding is the whole
// of the binding step.
func bindJSON(w http.ResponseWriter, r *http.Request, obj any) error {
	if r == nil || r.Body == nil {
		abortWithStatus(w, http.StatusBadRequest)
		return errInvalidRequest
	}
	if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
		abortWithStatus(w, http.StatusBadRequest)
		return err
	}
	return nil
}
