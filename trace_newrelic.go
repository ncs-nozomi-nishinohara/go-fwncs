package fwncs

import (
	"context"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	newrelic "github.com/newrelic/go-agent/v3/newrelic"
)

// func init() {
// 	internal.TrackUsage("integration", "framework", PackageName, Version)
// }

const NewRelicAppKey = "newrelicApp"

type newrelicResponseWriter struct {
	ResponseWriter
	replacement http.ResponseWriter
	code        int
	written     bool
}

var _ ResponseWriter = &newrelicResponseWriter{}

func (w *newrelicResponseWriter) flushHeader() {
	if !w.written {
		w.replacement.WriteHeader(w.code)
		w.written = true
	}
}

func (w *newrelicResponseWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *newrelicResponseWriter) Write(data []byte) (int, error) {
	w.flushHeader()
	return w.ResponseWriter.Write(data)
}

func (w *newrelicResponseWriter) WriteString(s string) (int, error) {
	w.flushHeader()
	return w.ResponseWriter.WriteString(s)
}

func (w *newrelicResponseWriter) WriteHeaderNow() {
	w.flushHeader()
	w.ResponseWriter.WriteHeaderNow()
}

type newrelicMiddleware struct {
	router         *Router
	app            *newrelic.Application
	once           sync.Once
	transactionMap map[string]map[string]TransactionInfo
}

func Newrelic(r *Router, app *newrelic.Application) TraceHandler {
	m := newrelicMiddleware{
		router: r,
		app:    app,
	}
	return m.handler
}

func (m *newrelicMiddleware) handler(method, path string, h httprouter.Handle) (string, string, httprouter.Handle) {
	return method, path, func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		w = wrapResponseWriter(w)
		if m.app != nil {
			m.once.Do(func() {
				m.transactionMap = TransactionNameGenerator(m.router)
			})
			requestName := method + " " + path
			tx := m.app.StartTransaction(requestName)
			tx.SetWebRequestHTTP(req)
			defer tx.End()
			repl := &newrelicResponseWriter{
				ResponseWriter: w.(ResponseWriter),
				replacement:    tx.SetWebResponse(w.(ResponseWriter)),
				code:           http.StatusOK,
			}
			w = repl
			defer repl.flushHeader()
			*req = *req.WithContext(context.WithValue(req.Context(), NewRelicAppKey, tx))
		}
		h(w, req, p)
	}
}
