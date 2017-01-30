package core

import "github.com/prometheus/client_golang/prometheus"

const (
	namespace = "gorb" // For Prometheus metrics.
)

var (
	serviceHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_health",
		Help:      "TODO",
	}, []string{"lb_name", "host", "port", "protocol", "persistent", "method"})
	serviceBackends = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backends",
		Help:      "TODO",
	})
	serviceBackendUptimeTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "service_backend_uptime_total",
		Help:      "TODO",
	})
	serviceBackendHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_health",
		Help:      "TODO",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
	serviceBackendStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_status",
		Help:      "TODO",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
	serviceBackendWeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_weight",
		Help:      "TODO",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
)

type Exporter struct {
	ctx *Context
}

func NewExporter(ctx *Context) *Exporter {
	return &Exporter{
		ctx: ctx,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	serviceHealth.Describe(ch)
	serviceBackends.Describe(ch)
	serviceBackendUptimeTotal.Describe(ch)
	serviceBackendHealth.Describe(ch)
	serviceBackendStatus.Describe(ch)
	serviceBackendWeight.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.ctx.mutex.RLock()
	defer e.ctx.mutex.RUnlock()
	// TODO - update metrics
}

func RegisterPrometheusExporter(ctx *Context) {
	prometheus.MustRegister(NewExporter(ctx))
}
