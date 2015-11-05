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
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type httpPulse struct {
	Pulse

	url      string
	interval time.Duration
	client   *http.Client
	stopChan chan struct{}
	metrics  *Metrics
}

// NewHTTPPulse creates a new httpPulse.
func NewHTTPPulse(address string, port uint16, opts *Options) Pulse {
	httpClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(
		req *http.Request,
		via []*http.Request,
	) error {
		if len(via) == 0 {
			return nil
		}

		return errors.New("redirects are not supported for pulse requests")
	}}

	return &httpPulse{
		url:      fmt.Sprintf("http://%s:%d/%s", address, port, opts.Path),
		interval: opts.interval,
		client:   httpClient,
		stopChan: make(chan struct{}, 1),
		metrics:  NewMetrics(),
	}
}

// Loop starts the httpPulse reactor.
func (p *httpPulse) Loop(id ID, pulseCh chan Status) {
	log.Infof("starting HTTP pulse for %s", p.url)

	for {
		select {
		case <-time.After(p.interval):
			msg := Status{id, StatusDown}

			if r, err := p.client.Get(p.url); err != nil {
				log.Errorf("error while communicating with %s: %s", p.url, err)
			} else if r.StatusCode != 200 {
				log.Errorf("received non-200 status code from %s", p.url)
			} else {
				msg.Result = StatusUp
			}

			// Report the backend status to context.
			pulseCh <- msg

			// Recalculate metrics and statistics.
			p.metrics.Update(msg)

		case <-p.stopChan:
			log.Infof("stopping HTTP pulse for %s", p.url)
			return
		}
	}
}

// Stop stops the httpPulse reactor.
func (p *httpPulse) Stop() {
	p.stopChan <- struct{}{}
}

// Info returns the httpPulse metrics.
func (p *httpPulse) Info() Metrics {
	return *p.metrics
}
