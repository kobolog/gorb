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

	"github.com/kobolog/gorb/util"

	log "github.com/sirupsen/logrus"
)

// Driver provides the actual health check for Pulse.
type Driver interface {
	Check() StatusType
}

var (
	get = map[string]func(string, uint16, util.DynamicMap) (Driver, error){
		"tcp":  newTCPDriver,
		"http": newGETDriver,
		"none": newNopDriver,
	}

	// Use a separate random device to avoid fucking with other packages.
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// Pulse is an health check manager for a backend.
type Pulse struct {
	driver   Driver
	interval time.Duration
	stopCh   chan struct{}
	metrics  *Metrics
}

// New creates a new Pulse from the provided endpoint and options.
func New(host string, port uint16, opts *Options) (*Pulse, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	d, err := get[opts.Type](host, port, opts.Args)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{}, 1)

	return &Pulse{d, opts.interval, stopCh, NewMetrics()}, nil
}

// Update is a Pulse notification message.
type Update struct {
	Source  ID
	Metrics Metrics
}

// Loop starts the Pulse.
func (p *Pulse) Loop(id ID, pulseCh chan Update) {
	log.Infof("starting pulse for %s", id)

	// Randomize the first health-check to avoid thundering herd syndrome.
	interval := time.Duration(rng.Int63n(int64(p.interval)))

	for {
		select {
		case <-time.After(interval):
			// Recalculate metrics and statistics and send them to Context.
			pulseCh <- Update{id, p.metrics.Update(p.driver.Check())}

		case <-p.stopCh:
			log.Infof("stopping pulse for %s", id)
			return
		}

		// TODO(@kobolog): Add exponential back-offs, thresholds.
		interval = p.interval

		log.Debugf("current pulse for %s: %s", p.metrics.Status.String())
	}
}

// Stop stops the Pulse.
func (p *Pulse) Stop() {
	p.stopCh <- struct{}{}
}
