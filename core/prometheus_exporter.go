package core

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "gorb" // For Prometheus metrics.
)

var (
	serviceHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_health",
		Help:      "Health of the load balancer service",
	}, []string{"name", "host", "port", "protocol", "method", "persistent"})

	serviceBackends = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backends",
		Help:      "Number of backends in the load balancer service",
	}, []string{"name", "host", "port", "protocol", "method", "persistent"})

	serviceBackendUptimeTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_uptime_seconds",
		Help:      "Uptime in seconds of a backend service",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_health",
		Help:      "Health of a backend service",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_status",
		Help:      "Status of a backend service",
	}, []string{"service_name", "name", "host", "port", "method"})

	serviceBackendWeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_backend_weight",
		Help:      "Weight of a backend service",
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
	serviceHealth.Describe(ch)
	serviceBackends.Describe(ch)
	serviceBackendUptimeTotal.Describe(ch)
	serviceBackendHealth.Describe(ch)
	serviceBackendStatus.Describe(ch)
	serviceBackendWeight.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	if err := e.collect(); err != nil {
		log.Errorf("error collecting metrics: %s", err)
		return
	}
	serviceHealth.Collect(ch)
	serviceBackends.Collect(ch)
	serviceBackendUptimeTotal.Collect(ch)
	serviceBackendHealth.Collect(ch)
	serviceBackendStatus.Collect(ch)
	serviceBackendWeight.Collect(ch)
}

func (e *Exporter) collect() error {
	e.ctx.mutex.RLock()
	defer e.ctx.mutex.RUnlock()

	for serviceName, _ := range e.ctx.services {
		service, err := e.ctx.GetService(serviceName)
		if err != nil {
			return errors.Wrap(err, "error getting service: %s", serviceName)
		}

		serviceHealth.WithLabelValues(serviceName, service.Options.Host, fmt.Sprintf("%d", service.Options.Port),
			service.Options.Protocol, service.Options.Method, fmt.Sprintf("%t", service.Options.Persistent)).
			Set(service.Health)

		serviceBackends.WithLabelValues(serviceName, service.Options.Host, fmt.Sprintf("%d", service.Options.Port),
			service.Options.Protocol, service.Options.Method, fmt.Sprintf("%t", service.Options.Persistent)).
			Set(float64(len(service.Backends)))

		for i := 0; i < len(service.Backends); i++ {
			backendName := service.Backends[i]
			backend, err := e.ctx.GetBackend(serviceName, backendName)
			if err != nil {
				return errors.Wrap(err, "error getting backend %s from service %s", backendName, serviceName)
			}

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
	return nil
}
func RegisterPrometheusExporter(ctx *Context) {
	prometheus.MustRegister(NewExporter(ctx))
}
