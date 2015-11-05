/*
    Copyright (c) 2015 Andrey Sibiryov <me@kobology.ru>
    Copyright (c) 2015 Other contributors as noted in the AUTHORS file.

    This file is part of GORB - Go Routing and Balancing.

    GORB is free software; you can redistribute it and/or modify
    it under the terms of the GNU Lesser General Public License as published by
    the Free Software Foundation; either version 3 of the License, or
    (at your option) any later version.

    Cocaine is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
    GNU Lesser General Public License for more details.

    You should have received a copy of the GNU Lesser General Public License
    along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package pulse

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

// tcpPulse checks a service health with TCP connection probes.
type tcpPulse struct {
	Pulse

	endpoint string
	interval time.Duration
	dialer   net.Dialer
	stopChan chan struct{}
	metrics  *Metrics
}

// NewTCPPulse creates a new tcpPulse.
func NewTCPPulse(address string, port uint16, opts *Options) Pulse {
	return &tcpPulse{
		endpoint: fmt.Sprintf("%s:%d", address, port),
		interval: opts.interval,
		dialer:   net.Dialer{DualStack: true, Timeout: 5 * time.Second},
		stopChan: make(chan struct{}, 1),
		metrics:  NewMetrics(),
	}
}

// Loop starts the tcpPulse reactor.
func (p *tcpPulse) Loop(id ID, pulseCh chan Status) {
	log.Infof("starting TCP pulse for %s", p.endpoint)

	for {
		select {
		case <-time.After(p.interval):
			msg := Status{id, StatusDown}

			if con, err := p.dialer.Dial("tcp", p.endpoint); err != nil {
				log.Errorf("unable to connect to %s", p.endpoint)
			} else {
				msg.Result = StatusUp
				con.Close()
			}

			// Report the backend status to context.
			pulseCh <- msg

			// Recalculate metrics and statistics.
			p.metrics.Update(msg)

		case <-p.stopChan:
			log.Infof("stopping TCP pulse for %s", p.endpoint)
			return
		}
	}
}

// Stop stops the tcpPulse reactor.
func (p *tcpPulse) Stop() {
	p.stopChan <- struct{}{}
}

// Info returns the tcpPulse metrics.
func (p *tcpPulse) Info() Metrics {
	return *p.metrics
}
