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
	"math/rand"
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
	stopCh   chan struct{}
	metrics  *Metrics
}

// New creates a new Pulse from the provided endpoint and options.
func New(address string, port uint16, opts *Options) *Pulse {
	var d Driver

	switch opts.Type {
	case "tcp":
		d = newTCPDriver(address, port, opts)
	case "http":
		d = newGETDriver(address, port, opts)
	case "none":
		d = newNopDriver()
	}

	return &Pulse{d, opts.interval, make(chan struct{}, 1), NewMetrics()}
}

// Use a separate random device to avoid interfering with other packages.
var rd *rand.Rand

// Loop starts the Pulse.
func (p *Pulse) Loop(id ID, pulseCh chan Update) {
	log.Infof("starting pulse for %s", id)

	// Randomize the first health-check to avoid thundering herd syndrome.
	interval := time.Duration(rd.Int63n(int64(p.interval)))

	for {
		select {
		case <-time.After(interval):
			status := Status{id, p.driver.Check()}

			// Recalculate metrics and statistics and send them to Context.
			pulseCh <- p.metrics.Update(status)

		case <-p.stopCh:
			log.Infof("stopping pulse for %s", id)
			return
		}

		// TODO(@kobolog): Add exponential back-offs, thresholds.
		interval = p.interval
	}
}

// Stop stops the Pulse.
func (p *Pulse) Stop() {
	p.stopCh <- struct{}{}
}

func init() {
	// NOTE(@kobolog): Docs say that UnixNano() is undefined if Unix time
	// cannot be represented as int64. Not sure what it mean, so whatever.
	rd = rand.New(rand.NewSource(time.Now().UnixNano()))
}
