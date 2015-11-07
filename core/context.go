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

package core

import (
	"errors"
	"sync"

	"github.com/kobolog/gorb/pulse"

	log "github.com/sirupsen/logrus"
	"github.com/tehnerd/gnl2go"
)

// Possible runtime errors.
var (
	ErrIpvsSyscallFailed = errors.New("error while calling into IPVS")
	ErrObjectExists      = errors.New("specified object already exists")
	ErrObjectNotFound    = errors.New("unable to locate specified object")
)

type service struct {
	options *ServiceOptions
}

type backend struct {
	options *BackendOptions
	service *service
	monitor *pulse.Pulse
}

// Context abstacts away the underlying IPVS bindings implementation.
type Context struct {
	ipvs     gnl2go.IpvsClient
	services map[string]*service
	backends map[string]*backend
	mtx      sync.RWMutex
	pulseCh  chan pulse.Status
}

// NewContext creates a new Context and initializes IPVS.
func NewContext(options ContextOptions) (*Context, error) {
	log.Info("initializing IPVS context")

	ctx := &Context{
		ipvs:     gnl2go.IpvsClient{},
		services: make(map[string]*service),
		backends: make(map[string]*backend),
		pulseCh:  make(chan pulse.Status),
	}

	if err := ctx.ipvs.Init(); err != nil {
		log.Errorf("unable to initialize IPVS context: %s", err)

		// Here and in other places: IPVS errors are abstracted to make GNL2GO
		// replaceable in the future, since it's not really maintained anymore.
		return nil, ErrIpvsSyscallFailed
	}

	if options.Flush && ctx.ipvs.Flush() != nil {
		log.Errorf("unable to clean up IPVS tables")
		ctx.Close()
		return nil, ErrIpvsSyscallFailed
	}

	// Fire off a pulse notifications sink goroutine.
	go pulseSink(ctx)

	return ctx, nil
}

// Close shuts down IPVS and closes the Context.
func (ctx *Context) Close() {
	log.Info("shutting down IPVS context")

	for vsID := range ctx.services {
		ctx.RemoveService(vsID)
	}

	// This will also shutdown the pulse notification sink goroutine.
	close(ctx.pulseCh)

	// This is not strictly required, as far as I know.
	ctx.ipvs.Exit()
}

// CreateService registers a new virtual service with IPVS.
func (ctx *Context) CreateService(vsID string, opts *ServiceOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	log.Infof("creating virtual service [%s] on %s:%d", vsID, opts.Address, opts.Port)

	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()

	if _, exists := ctx.services[vsID]; exists {
		log.Errorf("virtual service [%s] already exists", vsID)
		return ErrObjectExists
	}

	if err := ctx.ipvs.AddService(
		opts.Address,
		opts.Port,
		opts.protocol,
		opts.Method,
	); err != nil {
		log.Errorf("error while creating virtual service: %s", err)
		return ErrIpvsSyscallFailed
	}

	ctx.services[vsID] = &service{options: opts}

	return nil
}

// CreateBackend registers a new backend with a virtual service.
func (ctx *Context) CreateBackend(vsID, rsID string, opts *BackendOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	log.Infof("creating backend on %s:%d for virtual service [%s]",
		opts.Address,
		opts.Port,
		vsID)

	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()

	if _, exists := ctx.backends[rsID]; exists {
		log.Errorf("backend [%s/%s] already exists", vsID, rsID)
		return ErrObjectExists
	}

	vs, exists := ctx.services[vsID]

	if !exists {
		log.Errorf("unable to find parent virtual service [%s]", vsID)
		return ErrObjectNotFound
	}

	if err := ctx.ipvs.AddDestPort(
		vs.options.Address,
		vs.options.Port,
		opts.Address,
		opts.Port,
		vs.options.protocol,
		int32(opts.Weight),
		opts.method,
	); err != nil {
		log.Errorf("error while adding backend: %s", err)
		return ErrIpvsSyscallFailed
	}

	backend := &backend{options: opts, service: vs, monitor: pulse.New(
		opts.Address,
		opts.Port,
		opts.Pulse)}

	ctx.backends[rsID] = backend

	// Fire off the configured pulse goroutine and attach it to the Context.
	go backend.monitor.Loop(pulse.ID{vsID, rsID}, ctx.pulseCh)

	return nil
}

// UpdateBackend updates the specified backend's weight - other options are ignored.
func (ctx *Context) UpdateBackend(vsID, rsID string, opts *BackendOptions) (*BackendOptions, error) {
	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()

	rs, exists := ctx.backends[rsID]

	if !exists {
		log.Errorf("unable to find backend [%s/%s]", vsID, rsID)
		return nil, ErrObjectNotFound
	}

	if err := ctx.ipvs.UpdateDestPort(
		rs.service.options.Address,
		rs.service.options.Port,
		rs.options.Address,
		rs.options.Port,
		rs.service.options.protocol,
		int32(opts.Weight),
		rs.options.method,
	); err != nil {
		log.Errorf("error while updating backend [%s/%s]", vsID, rsID)
		return nil, ErrIpvsSyscallFailed
	}

	var result BackendOptions

	// Save the old backend options and update the current backend weight.
	result, rs.options.Weight = *rs.options, opts.Weight

	return &result, nil
}

// RemoveService deregisters a virtual service.
func (ctx *Context) RemoveService(vsID string) (*ServiceOptions, error) {
	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()

	vs, exists := ctx.services[vsID]

	if !exists {
		log.Errorf("unable to find virtual service [%s]", vsID)
		return nil, ErrObjectNotFound
	}

	for rsID, backend := range ctx.backends {
		if backend.service != vs {
			continue
		}

		// Stop the pulse goroutine.
		backend.monitor.Stop()

		delete(ctx.backends, rsID)
	}

	if err := ctx.ipvs.DelService(
		vs.options.Address,
		vs.options.Port,
		vs.options.protocol,
	); err != nil {
		log.Errorf("error while removing virtual service [%s]", vsID)
		return nil, ErrIpvsSyscallFailed
	}

	delete(ctx.services, vsID)

	return vs.options, nil
}

// RemoveBackend deregisters a backend.
func (ctx *Context) RemoveBackend(vsID, rsID string) (*BackendOptions, error) {
	ctx.mtx.Lock()
	defer ctx.mtx.Unlock()

	rs, exists := ctx.backends[rsID]

	if !exists {
		log.Errorf("unable to find backend [%s/%s]", vsID, rsID)
		return nil, ErrObjectNotFound
	}

	// Stop the pulse goroutine.
	rs.monitor.Stop()

	if err := ctx.ipvs.DelDestPort(
		rs.service.options.Address,
		rs.service.options.Port,
		rs.options.Address,
		rs.options.Port,
		rs.service.options.protocol,
	); err != nil {
		log.Errorf("error while removing backend [%s/%s]", vsID, rsID)
		return nil, ErrIpvsSyscallFailed
	}

	delete(ctx.backends, rsID)

	return rs.options, nil
}

// GetService returns information about a virtual service.
func (ctx *Context) GetService(vsID string) (*ServiceOptions, error) {
	ctx.mtx.RLock()
	vs, exists := ctx.services[vsID]
	ctx.mtx.RUnlock()

	if !exists {
		log.Errorf("unable to find virtual service [%s]", vsID)
		return nil, ErrObjectNotFound
	}

	return vs.options, nil
}

// BackendInfo contains information about backend options and pulse.
type BackendInfo struct {
	Options *BackendOptions `json:"options"`
	Pulse   pulse.Metrics   `json:"pulse"`
}

// GetBackend returns information about a backend.
func (ctx *Context) GetBackend(vsID, rsID string) (*BackendInfo, error) {
	ctx.mtx.RLock()
	rs, exists := ctx.backends[rsID]
	ctx.mtx.RUnlock()

	if !exists {
		log.Errorf("unable to find backend [%s]", rsID)
		return nil, ErrObjectNotFound
	}

	return &BackendInfo{rs.options, rs.monitor.Info()}, nil
}
