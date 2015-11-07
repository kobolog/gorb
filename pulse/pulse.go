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

package pulse

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// Driver provides the actual health check for Pulse.
type Driver interface {
	Check() StatusType
}

// Pulse is an health check manager for a backend.
type Pulse struct {
	driver   Driver
	interval time.Duration
	stopChan chan struct{}
	metrics  *Metrics
}

// New creates a new Pulse from the provided endpoint and options.
func New(address string, port uint16, opts *Options) *Pulse {
	var driver Driver

	switch opts.Type {
	case "tcp":
		driver = newTCPDriver(address, port, opts)
	case "http":
		driver = newHTTPDriver(address, port, opts)
	}

	return &Pulse{driver, opts.interval, make(chan struct{}, 1), NewMetrics()}
}

// Loop starts the Pulse.
func (p *Pulse) Loop(id ID, pulseCh chan Status) {
	log.Infof("starting pulse for %s", id)

	for {
		select {
		case <-time.After(p.interval):
			status := Status{id, p.driver.Check()}

			// Report the backend status to Context.
			pulseCh <- status

			// Recalculate metrics and statistics.
			p.metrics.Update(status)

		case <-p.stopChan:
			log.Infof("stopping pulse for %s", id)
			return
		}

		log.Debugf("%s pulse: %s", id, p.metrics.Status)
	}
}

// Stop stops the Pulse.
func (p *Pulse) Stop() {
	p.stopChan <- struct{}{}
}

// Info returns Pulse metrics and statistics.
func (p *Pulse) Info() Metrics {
	return *p.metrics
}
