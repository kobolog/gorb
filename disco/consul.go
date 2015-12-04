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

package disco

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/kobolog/gorb/util"
)

var (
	errConsulError = errors.New("error while calling into Consul")
)

type consulDisco struct {
	Driver

	client http.Client
	consul *url.URL
}

func newConsulDriver(opts util.DynamicMap) (Driver, error) {
	u, err := url.Parse(
		opts.Get("URL", "http://localhost:8500").(string))
	if err != nil {
		return nil, err
	}

	return &consulDisco{
		client: http.Client{Timeout: 5 * time.Second},
		consul: u,
	}, nil
}

type exposeRequest struct {
	Name string `json:"Name"`
	Host string `json:"Address"`
	Port uint16 `json:"Port"`
}

func (c *consulDisco) Expose(name, host string, port uint16) error {
	u := *c.consul
	u.Path = "v1/agent/service/register"

	r, err := c.client.Post(
		u.String(),
		"application/json",
		bytes.NewBuffer(util.MustMarshal(exposeRequest{
			Name: name,
			Host: host,
			Port: port,
		}, util.JSONOptions{})))
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return errConsulError
	}

	return nil
}

func (c *consulDisco) Remove(name string) error {
	u := *c.consul
	u.Path = path.Join("v1/agent/service/deregister", name)

	r, err := c.client.Get(u.String())
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return errConsulError
	}

	return nil
}
