package core

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "gorb" // For Prometheus metrics.
)

var (
	serviceHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_health",
		Help:      "TODO",
	}, []string{"name", "host", "port", "protocol", "method", "persistent"})

	serviceBackends = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backends",
		Help:      "TODO",
	}, []string{"name", "host", "port", "protocol", "method", "persistent"})

	serviceBackendUptimeTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_uptime_seconds",
		Help:      "TODO",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_health",
		Help:      "TODO",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_status",
		Help:      "TODO",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendWeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_weight",
		Help:      "TODO",
	}, []string{"service_name", "name", "host", "port", "method"})
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
	fmt.Printf("Describing metrics...\n")
	serviceHealth.Describe(ch)
	serviceBackends.Describe(ch)
	serviceBackendUptimeTotal.Describe(ch)
	serviceBackendHealth.Describe(ch)
	serviceBackendStatus.Describe(ch)
	serviceBackendWeight.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	fmt.Printf("Collecting metrics...\n")
	e.ctx.mutex.RLock()
	defer e.ctx.mutex.RUnlock()
	// TODO - update metrics
	for serviceName, _ := range e.ctx.services {
		service, err := e.ctx.GetService(serviceName)
		if err != nil {
			// do something about it
			fmt.Printf("Error getting service %s - %s\n", serviceName, err.Error())
		}
		fmt.Printf("serviceName %s - service %+v\n", serviceName, service)
		serviceHealth.WithLabelValues(serviceName, service.Options.Host, fmt.Sprintf("%d", service.Options.Port),
			service.Options.Protocol, service.Options.Method, fmt.Sprintf("%t", service.Options.Persistent)).
			Set(service.Health)
		serviceBackends.WithLabelValues(serviceName, service.Options.Host, fmt.Sprintf("%d", service.Options.Port),
			service.Options.Protocol, service.Options.Method, fmt.Sprintf("%t", service.Options.Persistent)).
			Set(float64(len(service.Backends)))
		fmt.Printf("Going to collect metric for %d backends\n", len(service.Backends))
		if len(service.Backends) == 0 {
			continue
		}
		for i := 0; i < len(service.Backends); i++ {
			backendName := service.Backends[i]
			backend, err := e.ctx.GetBackend(serviceName, backendName)
			if err != nil {
				// do something about it
				fmt.Printf("Error getting backend %s from service %s - %s\n", backendName, serviceName, err.Error())
			}
			fmt.Printf("backendName %s - backend %+v\n", backendName, backend)
			serviceBackendUptimeTotal.WithLabelValues(serviceName, backendName, backend.Options.Host,
				fmt.Sprintf("%d", backend.Options.Port), backend.Options.Method).
				Set(backend.Metrics.Uptime.Seconds())

			serviceBackendHealth.WithLabelValues(serviceName, backendName, backend.Options.Host,
				fmt.Sprintf("%d", backend.Options.Port), backend.Options.Method).
				Set(backend.Metrics.Health)

			serviceBackendStatus.WithLabelValues(serviceName, backendName, backend.Options.Host,
				fmt.Sprintf("%d", backend.Options.Port), backend.Options.Method).
				Set(float64(backend.Metrics.Status))

			serviceBackendWeight.WithLabelValues(serviceName, backendName, backend.Options.Host,
				fmt.Sprintf("%d", backend.Options.Port), backend.Options.Method).
				Set(float64(backend.Options.Weight))
		}

	}
	serviceHealth.Collect(ch)
	serviceBackends.Collect(ch)
	serviceBackendUptimeTotal.Collect(ch)
	serviceBackendHealth.Collect(ch)
	serviceBackendStatus.Collect(ch)
	serviceBackendWeight.Collect(ch)

}

func RegisterPrometheusExporter(ctx *Context) {
	prometheus.MustRegister(NewExporter(ctx))
}
