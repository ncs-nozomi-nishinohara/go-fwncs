package fwncs

import (
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/opentracing/opentracing-go"
)

type Builder struct {
	tracePrometheus    bool
	elastic            []ElasticOption
	newrelic           *newrelic.Application
	opentracingTracer  opentracing.Tracer
	opentracingOptions []OpentracingOption
	logger             ILogger
}

type Options func(builder *Builder)

func (o Options) Apply(builder *Builder) {
	o(builder)
}

func UsePrometheus() Options {
	return func(builder *Builder) {
		builder.tracePrometheus = true
	}
}

func UseElasticAPM(opts ...ElasticOption) Options {
	return func(builder *Builder) {
		builder.elastic = opts
	}
}

func UseNewrelic(app *newrelic.Application) Options {
	return func(builder *Builder) {
		builder.newrelic = app
	}
}

func UseOpentracing(tracer opentracing.Tracer, opts ...OpentracingOption) Options {
	if tracer == nil {
		tracer = getOpentracingTracer()
	}
	return func(builder *Builder) {
		builder.opentracingTracer = tracer
		builder.opentracingOptions = opts
	}
}

func LoggerOptions(log ILogger) Options {
	return func(builder *Builder) {
		builder.logger = log
	}
}
