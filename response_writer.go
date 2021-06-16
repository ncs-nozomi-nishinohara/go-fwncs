package fwncs

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

type (
	ResponseWriter interface {
		http.ResponseWriter
		http.Hijacker
		http.Flusher
		http.CloseNotifier

		Status() int
		Size() int
		WriteString(string) (int, error)
		Written() bool
		WriteHeaderNow()

		Pusher() http.Pusher
	}
	responseWriter struct {
		http.ResponseWriter
		status int
		size   int
		log    ILogger
	}
)

const noWritten = -1

func (w *responseWriter) Write(buf []byte) (n int, err error) {
	w.WriteHeaderNow()
	n, err = w.ResponseWriter.Write(buf)
	w.size += n
	return
}
func (w *responseWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.Written() {
			w.log.Debug("[WARNING] Headers were already written. Wanted to override status code %d with %d", w.status, code)
		}
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
		w.WriteHeader(w.status)
	}
}

func (w *responseWriter) WriteString(s string) (n int, err error) {
	w.WriteHeaderNow()
	n, err = io.WriteString(w.ResponseWriter, s)
	w.size += n
	return
}

func (w *responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.size != noWritten
}

// Hijack implements the http.Hijacker interface.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.size < 0 {
		w.size = 0
	}
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// CloseNotify implements the http.CloseNotify interface.
func (w *responseWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// Flush implements the http.Flush interface.
func (w *responseWriter) Flush() {
	w.WriteHeaderNow()
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *responseWriter) Pusher() (pusher http.Pusher) {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}

func wrapResponseWriter(w http.ResponseWriter) ResponseWriter {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	rw := &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		size:           noWritten,
	}
	return rw
}
