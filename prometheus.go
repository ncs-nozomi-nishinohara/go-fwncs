package fwncs

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"handler", "code", "method"},
	)

	duration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
		},
		[]string{"handler", "code", "method"},
	)

	responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{200, 500, 900, 1500},
		},
		[]string{"handler", "code", "method"},
	)
)

func init() {
	prometheus.MustRegister(inFlight, counter, duration, responseSize)
}

func InstrumentHandlerInFlight(c Context) {
	inFlight.Inc()
	defer inFlight.Dec()
	c.Next()
}

func InstrumentHandlerDuration(c Context) {
	now := time.Now()
	req := c.Request()
	method := req.Method
	name := req.URL.Path
	c.Next()
	status := c.GetStatus()
	label := prometheus.Labels{
		"handler": name,
		"code":    strconv.Itoa(status),
		"method":  method,
	}
	go duration.With(label).Observe(time.Since(now).Seconds())
}

func InstrumentHandlerCounter(c Context) {
	req := c.Request()
	method := req.Method
	name := req.URL.Path
	c.Next()
	status := c.GetStatus()
	label := prometheus.Labels{
		"handler": name,
		"code":    strconv.Itoa(status),
		"method":  method,
	}
	go counter.With(label).Inc()
}

func InstrumentHandlerResponseSize(c Context) {
	req := c.Request()
	method := req.Method
	name := req.URL.Path
	c.Next()
	status := c.GetStatus()
	label := prometheus.Labels{
		"handler": name,
		"code":    strconv.Itoa(status),
		"method":  method,
	}
	go responseSize.With(label).Observe(float64(c.ResponseSize()))
}
