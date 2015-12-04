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

	"github.com/kobolog/gorb/util"

	log "github.com/sirupsen/logrus"
)

type httpPulse struct {
	Driver

	client http.Client
	httpRq *http.Request
	expect int
}

func newGETDriver(host string, port uint16, opts util.DynamicMap) (Driver, error) {
	c := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(
		req *http.Request,
		via []*http.Request,
	) error {
		return errors.New("redirects are not supported for pulse requests")
	}}

	u := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   opts.Get("path", "/").(string)}

	r, err := http.NewRequest(opts.Get("method", "GET").(string), u.String(), nil)
	if err != nil {
		return nil, err
	}

	return &httpPulse{
		client: c,
		httpRq: r,
		expect: opts.Get("expect", 200).(int),
	}, nil
}

func (p *httpPulse) Check() StatusType {
	if r, err := p.client.Do(p.httpRq); err != nil {
		log.Errorf("error while communicating with %s: %s", p.httpRq.URL, err)
	} else if r.StatusCode != p.expect {
		log.Errorf("received non-%d status code from %s", p.expect, p.httpRq.URL)
	} else {
		return StatusUp
	}

	return StatusDown
}
