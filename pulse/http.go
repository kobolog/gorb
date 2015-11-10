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

	method string
	url    string
	client http.Client
}

func newGETDriver(host string, port uint16, opts *Options) Driver {
	httpClient := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(
		req *http.Request,
		via []*http.Request,
	) error {
		if len(via) == 0 {
			return nil
		}

		return errors.New("redirects are not supported for pulse requests")
	}}

	url := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   opts.Path,
	}

	return &httpPulse{method: "GET", url: url.String(), client: httpClient}
}

func (p *httpPulse) Check() StatusType {
	if req, err := http.NewRequest(p.method, p.url, nil); err != nil {
		log.Errorf("error while creating a request: %s", err)
	} else if r, err := p.client.Do(req); err != nil {
		log.Errorf("error while communicating with %s: %s", p.url, err)
	} else if r.StatusCode != 200 {
		log.Errorf("received non-200 status code from %s", p.url)
	} else {
		return StatusUp
	}

	return StatusDown
}
