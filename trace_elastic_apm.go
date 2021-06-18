package fwncs

import (
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"go.elastic.co/apm/stacktrace"
)

func init() {
	stacktrace.RegisterLibraryPackage(
		PackageName,
	)
}

type elasticMiddleware struct {
	router         *Router
	tracer         *apm.Tracer
	requestIgnore  apmhttp.RequestIgnorerFunc
	once           sync.Once
	transactionMap map[string]map[string]TransactionInfo
}

type ElasticOption func(*elasticMiddleware)

func Elastic(r *Router, opts ...ElasticOption) TraceHandler {
	m := &elasticMiddleware{
		tracer: apm.DefaultTracer,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m.handler
}

func (m *elasticMiddleware) handler(method, path string, h httprouter.Handle) (string, string, httprouter.Handle) {
	return method, path, func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		w = wrapResponseWriter(w, m.router.logger)
		if !m.tracer.Recording() || m.requestIgnore(req) {
			return
		}
		m.once.Do(func() {
			m.transactionMap = TransactionNameGenerator(m.router)
		})
		requestName := method + " " + path
		tx, body, r := apmhttp.StartTransactionWithBody(m.tracer, requestName, req)
		defer tx.End()
		*req = *r
		defer func() {
			if v := recover(); v != nil {
				w.WriteHeader(http.StatusInternalServerError)
				ec := m.tracer.Recovered(v)
				ec.SetTransaction(tx)
				setElasticContext(&ec.Context, req, w.(*responseWriter), body)
				ec.Send()
			}
			tx.Result = apmhttp.StatusCodeResult(w.(*responseWriter).Status())
			if tx.Sampled() {
				setElasticContext(&tx.Context, req, w.(*responseWriter), body)
			}
			body.Discard()
		}()
		h(w, req, p)
	}
}

func setElasticContext(ctx *apm.Context, req *http.Request, w ResponseWriter, body *apm.BodyCapturer) {
	ctx.SetFramework("fwncs", Version)
	ctx.SetHTTPRequest(req)
	ctx.SetHTTPRequestBody(body)
	ctx.SetHTTPStatusCode(w.Status())
	ctx.SetHTTPResponseHeaders(w.Header())
}

func WithTracer(t *apm.Tracer) ElasticOption {
	if t == nil {
		panic("t == nil")
	}
	return func(m *elasticMiddleware) {
		m.tracer = t
	}
}

func WithRequestIgnorer(r apmhttp.RequestIgnorerFunc) ElasticOption {
	if r == nil {
		r = apmhttp.IgnoreNone
	}
	return func(m *elasticMiddleware) {
		m.requestIgnore = r
	}
}
