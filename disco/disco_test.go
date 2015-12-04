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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kobolog/gorb/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsulDriver(t *testing.T) {
	tests := []struct {
		op func(cd Driver) error
		fn func(w http.ResponseWriter, r *http.Request)
		rv error
	}{
		{
			// Normal response for Expose().
			func(cd Driver) error {
				return cd.Expose("name", "host", 1024)
			},
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, "/v1/agent/service/register", r.URL.RequestURI())

				var req exposeRequest
				err := json.NewDecoder(r.Body).Decode(&req)

				// Make sure that we send the proper request.
				require.NoError(t, err)
				require.Equal(t, exposeRequest{Name: "name", Host: "host", Port: 1024}, req)
			},
			nil,
		},
		{
			// Non-200 response code for Expose().
			func(cd Driver) error {
				return cd.Expose("name", "host", 1024)
			},
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			errUnexpectedResponse,
		},
		{
			// Normal response code for Remove().
			func(cd Driver) error {
				return cd.Remove("name")
			},
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/v1/agent/service/deregister/name", r.URL.RequestURI())
			},
			nil,
		},
		{
			// Non-200 response code for Expose().
			func(cd Driver) error {
				return cd.Remove("name")
			},
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			errUnexpectedResponse,
		},
	}

	for _, test := range tests {
		ts := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				test.fn(w, r)
			}))

		cd, err := New(&Options{Type: "consul", Args: util.DynamicMap{"URL": ts.URL}})
		require.NoError(t, err)

		assert.Equal(t, test.rv, test.op(cd))
	}
}

func TestConsulDriverInvalidURL(t *testing.T) {
	cd, err := New(&Options{Type: "consul", Args: util.DynamicMap{"URL": "dog@mail.com"}})
	require.NoError(t, err)

	// Make sure the driver fails with broken Consul URLs.
	require.Error(t, cd.Expose("name", "host", 1024))
	require.Error(t, cd.Remove("name"))
}
