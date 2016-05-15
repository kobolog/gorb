/*
   Copyright (c) 2015 Andrey Sibiryov <me@kobology.ru>
   Copyright (c) 2015 Other contributors as noted in the AUTHORS file.

   This file is part of GORB - Go Routing and Balancing.

   GORB is free software; you can redistribute it and/or modify
   it under the terms of the GNU Lesser General Public License as published by
   the Free Software Foundation; either version 3 of the License, or
   (at your option) any later version.

   GORB is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
   GNU Lesser General Public License for more details.

   You should have received a copy of the GNU Lesser General Public License
   along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package core

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	promNamespace = "gorb"
	defaultSleep  = time.Second * 5
)

var (
	lbHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "lbs_health",
		Help:      "load balancers health percentage",
	}, []string{"lb_name", "host", "port", "protocol", "persistent", "method"})
	backendNumber = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "lbs_backends_number",
		Help:      "load balancers backends informations",
	}, []string{"lb_name", "host", "port", "method"})
	backendWeight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "lbs_backends_weight",
		Help:      "load balancers backends weight",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
	backendStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "lbs_backends_status",
		Help:      "load balancers backends status",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
	backendHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: promNamespace,
		Name:      "lbs_backends_health",
		Help:      "load balancers backends health",
	}, []string{"lb_name", "backend_name", "host", "port", "method"})
	lbChannelMap      = make(map[string]chan int)
	backendChannelMap = make(map[string]chan int)
)

// Prometheus time series
func init() {
	prometheus.MustRegister(lbHealth)
	prometheus.MustRegister(backendNumber)
	prometheus.MustRegister(backendWeight)
	prometheus.MustRegister(backendStatus)
	prometheus.MustRegister(backendHealth)
}

func StartLBMetric(ctx *Context, vsID string) {
	lbChannelMap[vsID] = make(chan int)
	for {
		select {
		case _ = <-lbChannelMap[vsID]:
			return
		default:
			service, err := ctx.GetService(vsID)
			if err != nil {
				log.Errorf("Error retrieving service %s: %s", vsID, err)
			} else {
				lbHealth.WithLabelValues(vsID, service.Options.Host, fmt.Sprintf("%v", service.Options.Port), service.Options.Protocol, strconv.FormatBool(service.Options.Persistent), service.Options.Method).Set(service.Health)
			}
			time.Sleep(defaultSleep)
		}
	}
}

func StartBackendMetric(ctx *Context, vsID, rsID string) {
	backendChannelMap[rsID] = make(chan int)
	rs, exists := ctx.backends[rsID]
	if !exists {
		log.Errorf("Error retrieving backend %s", rsID)
	} else {
		backendNumber.WithLabelValues(vsID, rs.service.options.Host, fmt.Sprintf("%v", rs.service.options.Port), rs.service.options.Method).Inc()
		for {
			select {
			case _ = <-backendChannelMap[rsID]:
				return
			default:
				// Update backend health every 5 seconds
				backend, err := ctx.GetBackend(vsID, rsID)
				if err != nil {
					log.Errorf("Error retrieving service %s or backend %s: %s", vsID, rsID, err)
				} else {
					backendStatus.WithLabelValues(vsID, rsID, backend.Options.Host, fmt.Sprintf("%v", backend.Options.Port), backend.Options.Method).Set(float64(backend.Metrics.Status))
					backendHealth.WithLabelValues(vsID, rsID, backend.Options.Host, fmt.Sprintf("%v", backend.Options.Port), backend.Options.Method).Set(backend.Metrics.Health)
					backendWeight.WithLabelValues(vsID, rsID, backend.Options.Host, fmt.Sprintf("%v", backend.Options.Port), backend.Options.Method).Set(float64(backend.Options.Weight))
				}
				time.Sleep(defaultSleep)
			}
		}
	}
}

func StopLBMetric(ctx *Context, vsID string) {
	service, err := ctx.GetService(vsID)
	if err != nil {
		log.Errorf("Error retrieving service %s: %s", vsID, err)
	} else {
		if ch, found := lbChannelMap[vsID]; found {
			ch <- 0
			delete(lbChannelMap, vsID)
			lbHealth.Delete(prometheus.Labels{"lb_name": vsID, "host": service.Options.Host, "port": fmt.Sprintf("%v", service.Options.Port), "method": service.Options.Method, "protocol": service.Options.Protocol, "persistent": strconv.FormatBool(service.Options.Persistent)})
		} else {
			log.Errorf("%s not found inside lbChannelMap keys", vsID)
		}
	}
}

func StopBackendMetric(ctx *Context, vsID, rsID string) {
	rs, exists := ctx.backends[rsID]
	if !exists {
		log.Errorf("Error retrieving backend %s", rsID)
	} else {
		if ch, found := backendChannelMap[rsID]; found {
			ch <- 0
			delete(backendChannelMap, rsID)
			backendNumber.WithLabelValues(vsID, rs.service.options.Host, fmt.Sprintf("%v", rs.service.options.Port), rs.service.options.Method).Dec()
			backendHealth.Delete(prometheus.Labels{"lb_name": vsID, "backend_name": rsID, "host": rs.options.Host, "port": fmt.Sprintf("%v", rs.options.Port), "method": rs.options.Method})
			backendStatus.Delete(prometheus.Labels{"lb_name": vsID, "backend_name": rsID, "host": rs.options.Host, "port": fmt.Sprintf("%v", rs.options.Port), "method": rs.options.Method})
			backendWeight.Delete(prometheus.Labels{"lb_name": vsID, "backend_name": rsID, "host": rs.options.Host, "port": fmt.Sprintf("%v", rs.options.Port), "method": rs.options.Method})
		} else {
			log.Errorf("%s not found inside backendChannelMap keys", rsID)
		}
	}
}
