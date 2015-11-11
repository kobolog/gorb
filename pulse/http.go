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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

type httpPulse struct {
	Driver

	client http.Client
	cooked *http.Request
}

func newGETDriver(host string, port uint16, opts *Options) (Driver, error) {
	httpClient := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(
		req *http.Request,
		via []*http.Request,
	) error {
		return errors.New("redirects aren't supported for pulse request")
	}}

	target := url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", host, port),
		Path: opts.Path}

	cooked, err := http.NewRequest("GET", target.String(), nil)
	if err != nil {
		return nil, err
	}

	return &httpPulse{client: httpClient, cooked: cooked}, nil
}

func (p *httpPulse) Check() StatusType {
	if r, err := p.client.Do(p.cooked); err != nil {
		log.Errorf("error while communicating with %s: %s", p.cooked.URL, err)
	} else if r.StatusCode != 200 {
		log.Errorf("received non-200 status code from %s", p.cooked.URL)
	} else {
		return StatusUp
	}

	return StatusDown
}
