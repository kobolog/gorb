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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericOptions(t *testing.T) {
	var (
		opts *Options
		err  error
	)

	// Automatic defaults – must fill only in omitted fields.
	tests := []struct {
		in *Options
		rv *Options
	}{
		{
			&Options{},
			&Options{Type: "tcp", Interval: "1m"},
		},
		{
			&Options{Type: "http", Path: "/hp"},
			&Options{Type: "http", Interval: "1m", Path: "/hp"},
		},
		{
			&Options{Interval: "5s"},
			&Options{Type: "tcp", Interval: "5s"},
		},
	}

	for _, test := range tests {
		err, _ = test.in.Validate(), test.rv.Validate()

		require.NoError(t, err)

		assert.Equal(t, test.rv.Type, test.in.Type)
		assert.Equal(t, test.rv.Interval, test.in.Interval)
		assert.Equal(t, test.rv.Path, test.in.Path)
		assert.Equal(t, test.rv.interval, test.in.interval)
	}

	// Invalid type.
	opts = &Options{Type: "invalid"}
	err = opts.Validate()

	require.Error(t, err)
	require.Equal(t, ErrUnknownPulseType, err)

	// Non-parsable interval.
	opts = &Options{Type: "tcp", Interval: "60"}
	err = opts.Validate()

	require.Error(t, err)

	// Non-positive interval.
	opts = &Options{Type: "tcp", Interval: "-5s"}
	err = opts.Validate()

	require.Error(t, err)
	require.Equal(t, ErrInvalidPulseInterval, err)
}

func TestGETDriverOptions(t *testing.T) {
	// Missing HTTP path.
	opts := &Options{Type: "http", Interval: "1s"}
	err := opts.Validate()

	require.Error(t, err)
	require.Equal(t, ErrMissingHTTPPulsePath, err)
}

func TestPulseChannel(t *testing.T) {
	opts := &Options{Type: "none", Interval: "1s"}
	opts.Validate()

	var (
		p         = New("none", 80, opts)
		pulseChan = make(chan Update)
		id        = ID{"VsID", "rsID"}
	)

	defer close(pulseChan)

	go p.Loop(id, pulseChan)

	update := <-pulseChan

	assert.Equal(t, id, update.Source)
	assert.Equal(t, StatusUp, update.Metrics.Status)
	assert.Equal(t, 1.0, update.Metrics.Health)

	// TODO(@kobolog): Make it some reasonable value.
	assert.Zero(t, update.Metrics.Uptime)
}

func TestPulseStop(t *testing.T) {
	opts := &Options{Type: "none", Interval: "1s"}
	opts.Validate()

	var (
		p  = New("none", 80, opts)
		wg sync.WaitGroup
	)

	wg.Add(1)

	go func() {
		p.Loop(ID{"VsID", "rsID"}, make(chan Update))
		wg.Done()
	}()

	p.Stop()

	// In theory, this can hang the test forever.
	wg.Wait()
}

func TestNopDriver(t *testing.T) {
	driver := newNopDriver()

	require.NotNil(t, driver)
	assert.Equal(t, StatusUp, driver.Check())
}

func TestTCPDriver(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	go func() {
		cn, err := ln.Accept()

		// TODO(@kobolog): Not sure whether it's usable in goroutines.
		require.NoError(t, err)

		cn.Close()
		ln.Close()
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	driver := newTCPDriver("localhost", uint16(port), &Options{})

	assert.Equal(t, StatusUp, driver.Check())

	// This will fail since listener accepted only one connection.
	assert.Equal(t, StatusDown, driver.Check())
}

func TestGETDriver(t *testing.T) {
	server := httptest.NewServer(
      http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok!")
	}))

	port := server.Listener.Addr().(*net.TCPAddr).Port
	driver := newGETDriver("localhost", uint16(port), &Options{Path: "/"})

	assert.Equal(t, StatusUp, driver.Check())

	// Kill the test HTTP server to verify Pulse failure.
	server.Close()

	assert.Equal(t, StatusDown, driver.Check())
}
