package fwncs

type Builder struct {
	logger ILogger
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
