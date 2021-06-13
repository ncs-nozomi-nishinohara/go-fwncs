package fwncs

import (
	"os"

	"github.com/newrelic/go-agent/v3/newrelic"
	"go.elastic.co/apm/module/apmhttprouter"
)

const (
	_prometheus = "prometheus"
	_newrelic   = "newrelic"
	elastic     = "elastic"
)

type Builder map[string]interface{}

type Options func(builder Builder)

func (o Options) Apply(builder Builder) {
	o(builder)
}

func NewrelicOptions(app *newrelic.Application) Options {
	return func(builder Builder) {
		builder[_newrelic] = app
	}
}

func ElasticAPMOptions(o ...apmhttprouter.Option) Options {
	return func(builder Builder) {
		builder[elastic] = o
	}
}

func PrometheusOptions() Options {
	return func(builder Builder) {
		builder[_prometheus] = true
	}
}

func defaultNewrelicApp() *newrelic.Application {
	NEW_RELIC_APP_NAME := os.Getenv("NEW_RELIC_APP_NAME")
	NEW_RELIC_LICENSE_KEY := os.Getenv("NEW_RELIC_LICENSE_KEY")
	app, _ := newrelic.NewApplication(
		newrelic.ConfigAppName(NEW_RELIC_APP_NAME),
		newrelic.ConfigLicense(NEW_RELIC_LICENSE_KEY),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	return app
}
