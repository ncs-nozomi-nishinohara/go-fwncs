package fwncs

import (
	"net/http"
)

type Builder struct {
	logger                  ILogger
	globalOPTIONS           http.Handler
	methodNotAllowedHandler http.Handler
}

type Options func(builder *Builder)

func (o Options) Apply(builder *Builder) {
	o(builder)
}

func LoggerOptions(log ILogger) Options {
	return func(builder *Builder) {
		builder.logger = log
	}
}

func MethodNotAllowed(handler http.Handler) Options {
	if handler == nil {
		handler = methodNotAllowed
	}
	return func(builder *Builder) {
		builder.methodNotAllowedHandler = handler
	}
}

func GlobalOPTIONS(handler http.Handler) Options {
	return func(builder *Builder) {
		builder.globalOPTIONS = handler
	}
}

func DefaultGlobalOPTIONS() Options {
	return func(builder *Builder) {
		builder.globalOPTIONS = defaultGlobalOPTIONS
	}
}
