package fwncs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go/config"
)

func NewJaegertracing() (io.Closer, error) {
	defcfg := config.Configuration{
		ServiceName: "fwncs-tracer",
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
		},
	}
	cfg, err := defcfg.FromEnv()
	if err != nil {
		return nil, err
	}
	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		return nil, err
	}
	opentracing.SetGlobalTracer(tracer)
	return closer, nil
}

type opentracingOptions struct {
	spanObserver func(span opentracing.Span, r *http.Request)
}

type OpentracingOption func(*opentracingOptions)

func OpentracingSpanObserver(f func(span opentracing.Span, r *http.Request)) OpentracingOption {
	return func(options *opentracingOptions) {
		options.spanObserver = f
	}
}

type opentracingMiddleware struct {
	router         *Router
	once           sync.Once
	transactionMap map[string]map[string]TransactionInfo
	options        *opentracingOptions
	tr             opentracing.Tracer
}

func (m *opentracingMiddleware) handler(method, path string, h httprouter.Handle) (string, string, httprouter.Handle) {
	return method, path, func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		w = wrapResponseWriter(w)
		m.once.Do(func() {
			m.transactionMap = TransactionNameGenerator(m.router)
		})
		var sp opentracing.Span
		requestName := method + " " + path
		carrier := opentracing.HTTPHeadersCarrier(req.Header)
		ctx, _ := m.tr.Extract(opentracing.HTTPHeaders, carrier)
		op := requestName
		sp = m.tr.StartSpan(op, ext.RPCServerOption(ctx))
		ext.HTTPMethod.Set(sp, req.Method)
		ext.HTTPUrl.Set(sp, req.URL.String())
		m.options.spanObserver(sp, req)
		ext.Component.Set(sp, PackageName)
		*req = *req.WithContext(opentracing.ContextWithSpan(req.Context(), sp))
		h(w, req, p)
		ext.HTTPStatusCode.Set(sp, uint16(w.(ResponseWriter).Status()))
		go sp.Finish()
	}
}

func JaegerMiddleware(router *Router, tr opentracing.Tracer, options ...OpentracingOption) TraceHandler {
	opts := &opentracingOptions{
		spanObserver: func(span opentracing.Span, r *http.Request) {},
	}

	for _, o := range options {
		o(opts)
	}
	m := &opentracingMiddleware{
		router:  router,
		tr:      tr,
		options: opts,
	}
	return m.handler
}

func CreateChildSpan(ctx context.Context, name string) opentracing.Span {
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan == nil {
		parentSpan = opentracing.StartSpan(name)
	}
	sp := opentracing.StartSpan(name, opentracing.ChildOf(parentSpan.Context()))
	sp.SetTag("name", name)
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	callerDetails := fmt.Sprintf("%s - %s#%d", frame.Function, frame.File, frame.Line)
	sp.SetTag("caller", callerDetails)
	return sp
}

func getOpentracingTracer() opentracing.Tracer {
	return opentracing.GlobalTracer()
}
