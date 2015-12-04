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
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericOptions(t *testing.T) {
	var (
		opts *Options
		err  error
	)

	// Automatic defaults – must fill in only omitted fields.
	tests := []struct {
		in *Options
		rv *Options
	}{
		{
			&Options{},
			&Options{Type: "tcp", Interval: "1m"},
		},
		{
			&Options{Type: "http"},
			&Options{Type: "http", Interval: "1m"},
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
		assert.Equal(t, test.rv.Args, test.in.Args)
		assert.Equal(t, test.rv.interval, test.in.interval)
	}

	// Invalid type.
	opts = &Options{Type: "invalid"}
	err = opts.Validate()

	require.Error(t, err)
	assert.Equal(t, ErrUnknownPulseType, err)

	// Non-parsable interval.
	opts = &Options{Type: "tcp", Interval: "60"}
	err = opts.Validate()

	require.Error(t, err)

	// Non-positive interval.
	opts = &Options{Type: "tcp", Interval: "-5s"}
	err = opts.Validate()

	require.Error(t, err)
	assert.Equal(t, ErrInvalidPulseInterval, err)

	// pulse.New() validating options.
	_, err = New("host", 80, &Options{Type: "unknown-driver"})

	require.Error(t, err)
	assert.Equal(t, ErrUnknownPulseType, err)
}

func TestMetrics(t *testing.T) {
	m := NewMetrics()

	// Record rollover.
	for i := 0; i <= 100; i++ {
		m.Update(StatusUp)
	}

	assert.Equal(t, 100, len(m.record))

	// Uptime switch.
	m.Update(StatusDown)

	assert.Equal(t, time.Duration(0), m.Uptime)
}

func TestPulseChannel(t *testing.T) {
	var (
		pulseCh = make(chan Update)
		id      = ID{"VsID", "rsID"}
	)

	defer close(pulseCh)

	bp, err := New("", 0, &Options{Type: "none", Interval: "1s"})
	require.NoError(t, err)

	go bp.Loop(id, pulseCh)
	defer bp.Stop()

	update := <-pulseCh

	assert.Equal(t, id, update.Source)
	assert.Equal(t, StatusUp, update.Metrics.Status)
	assert.Equal(t, 1.0, update.Metrics.Health)

	// TODO(@kobolog): Make it some reasonable value.
	assert.Zero(t, update.Metrics.Uptime)
}

func TestPulseStop(t *testing.T) {
	var (
		pulseCh = make(chan Update)
		wg      sync.WaitGroup
	)

	defer close(pulseCh)

	bp, err := New("unknown-host", 80, &Options{Type: "tcp", Interval: "1s"})
	require.NoError(t, err)

	wg.Add(1)
	go func() {
		bp.Loop(ID{"VsID", "rsID"}, pulseCh)
		wg.Done()
	}()

	// Cover the pulse.StatusDown.String() case.
	<-pulseCh

	// In theory, this can hang the test forever.
	bp.Stop()
	wg.Wait()
}

func TestNopDriver(t *testing.T) {
	bp, err := New("", 0, &Options{Type: "none"})
	require.NoError(t, err)

	assert.Equal(t, StatusUp, bp.driver.Check())
}

func TestTCPDriver(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	go func() {
		cn, err := ln.Accept()
		defer cn.Close()

		// TODO(@kobolog): Not sure if it's usable in goroutines.
		require.NoError(t, err)
	}()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	bp, err := New("localhost", uint16(tcpAddr.Port), &Options{Type: "tcp"})
	require.NoError(t, err)

	// Normal connection attempt.
	assert.Equal(t, StatusUp, bp.driver.Check())

	ln.Close()

	// Connection failure.
	assert.Equal(t, StatusDown, bp.driver.Check())
}

func TestGETDriver(t *testing.T) {
	tests := []struct {
		fn func(w http.ResponseWriter, r *http.Request)
		rv StatusType
	}{
		{
			// Non-200 response code.
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			StatusDown,
		},
		{
			// Redirect.
			func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/url", http.StatusFound)
			},
			StatusDown,
		},
		{
			// Normal response.
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			StatusUp,
		},
	}

	for _, test := range tests {
		ts := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				test.fn(w, r)
			}))

		tcpAddr := ts.Listener.Addr().(*net.TCPAddr)
		bp, err := New("localhost", uint16(tcpAddr.Port), &Options{Type: "http"})
		require.NoError(t, err)

		assert.Equal(t, test.rv, bp.driver.Check())
	}
}

func TestGETDriverInvalidURL(t *testing.T) {
	_, err := New("dog@mail.com", 80, &Options{Type: "http"})
	require.Error(t, err)
}

func TestGETDriverNoConnection(t *testing.T) {
	bp, err := New("unknown-host", 80, &Options{Type: "http"})
	require.NoError(t, err)

	// Connection failure.
	assert.Equal(t, StatusDown, bp.driver.Check())
}
