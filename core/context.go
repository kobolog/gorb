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
	"fmt"
	"net"
	"sync"

	"github.com/kobolog/gorb/disco"
	"github.com/kobolog/gorb/pulse"
	"github.com/kobolog/gorb/util"
	"github.com/vishvananda/netlink"

	log "github.com/Sirupsen/logrus"
	"github.com/tehnerd/gnl2go"
	"strings"
)

// Possible runtime errors.
var (
	schedulerFlags = map[string]int{
		"sh-fallback": gnl2go.IP_VS_SVC_F_SCHED_SH_FALLBACK,
		"sh-port": gnl2go.IP_VS_SVC_F_SCHED_SH_PORT,
	}
	ErrIpvsSyscallFailed = errors.New("error while calling into IPVS")
	ErrObjectExists = errors.New("specified object already exists")
	ErrObjectNotFound = errors.New("unable to locate specified object")
	ErrIncompatibleAFs = errors.New("incompatible address families")
)

type service struct {
	options *ServiceOptions
}

type backend struct {
	options *BackendOptions
	service *service
	monitor *pulse.Pulse
	metrics pulse.Metrics
}

// Context abstacts away the underlying IPVS bindings implementation.
type Context struct {
	ipvs         Ipvs
	endpoint     net.IP
	services     map[string]*service
	backends     map[string]*backend
	mutex        sync.RWMutex
	pulseCh      chan pulse.Update
	disco        disco.Driver
	stopCh       chan struct{}
	vipInterface netlink.Link
	store        *Store
}

type Ipvs interface {
	Init() error
	Exit()
	Flush() error
	AddService(vip string, port uint16, protocol uint16, sched string) error
	AddServiceWithFlags(vip string, port uint16, protocol uint16, sched string, flags []byte) error
	DelService(vip string, port uint16, protocol uint16) error
	AddDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16, weight int32, fwd uint32) error
	UpdateDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16, weight int32, fwd uint32) error
	DelDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16) error
}

// NewContext creates a new Context and initializes IPVS.
func NewContext(options ContextOptions) (*Context, error) {
	log.Info("initializing IPVS context")

	ctx := &Context{
		ipvs:     &gnl2go.IpvsClient{},
		services: make(map[string]*service),
		backends: make(map[string]*backend),
		pulseCh:  make(chan pulse.Update),
		stopCh:   make(chan struct{}),
	}

	if len(options.Disco) > 0 {
		log.Infof("creating Consul client with Agent URL: %s", options.Disco)

		var err error

		ctx.disco, err = disco.New(&disco.Options{
			Type: "consul",
			Args: util.DynamicMap{"URL": options.Disco}})

		if err != nil {
			return nil, err
		}
	} else {
		ctx.disco, _ = disco.New(&disco.Options{Type: "none"})
	}

	if len(options.Endpoints) > 0 {
		// TODO(@kobolog): Bind virtual services on multiple endpoints.
		ctx.endpoint = options.Endpoints[0]
		if options.ListenPort != 0 {
			log.Info("Registered the REST service to Consul.")
			ctx.disco.Expose("gorb", ctx.endpoint.String(), options.ListenPort)
		}
	}

	if err := ctx.ipvs.Init(); err != nil {
		log.Errorf("unable to initialize IPVS context: %s", err)

		// Here and in other places: IPVS errors are abstracted to make GNL2GO
		// replaceable in the future, since it's not really maintained anymore.
		return nil, ErrIpvsSyscallFailed
	}

	if options.Flush && ctx.ipvs.Flush() != nil {
		log.Errorf("unable to clean up IPVS pools - ensure ip_vs is loaded")
		ctx.Close()
		return nil, ErrIpvsSyscallFailed
	}

	if options.VipInterface != "" {
		var err error
		if ctx.vipInterface, err = netlink.LinkByName(options.VipInterface); err != nil {
			ctx.Close()
			return nil, fmt.Errorf(
				"unable to find the interface '%s' for VIPs: %s",
				options.VipInterface, err)
		}
		log.Infof("VIPs will be added to interface '%s'", ctx.vipInterface.Attrs().Name)
	}

	// Fire off a pulse notifications sink goroutine.
	go ctx.run()

	return ctx, nil
}

// Close shuts down IPVS and closes the Context.
func (ctx *Context) Close() {
	log.Info("shutting down IPVS context")

	// This will also shutdown the pulse notification sink goroutine.
	close(ctx.stopCh)

	for vsID := range ctx.services {
		ctx.RemoveService(vsID)
	}

	// This is not strictly required, as far as I know.
	ctx.ipvs.Exit()
}

// CreateService registers a new virtual service with IPVS.
func (ctx *Context) createService(vsID string, opts *ServiceOptions) error {
	if err := opts.Validate(ctx.endpoint); err != nil {
		return err
	}

	if _, exists := ctx.services[vsID]; exists {
		return ErrObjectExists
	}

	if ctx.vipInterface != nil {
		ifName := ctx.vipInterface.Attrs().Name
		vip := &netlink.Addr{IPNet: &net.IPNet{
			net.ParseIP(opts.host.String()), net.IPv4Mask(255, 255, 255, 255)}}
		if err := netlink.AddrAdd(ctx.vipInterface, vip); err != nil {
			log.Infof(
				"failed to add VIP %s to interface '%s' for service [%s]: %s",
				opts.host, ifName, vsID, err)
		} else {
			opts.delIfAddr = true
		}
		log.Infof("VIP %s has been added to interface '%s'", opts.host, ifName)
	}

	log.Infof("creating virtual service [%s] on %s:%d", vsID, opts.host,
		opts.Port)

	// create service to external store
	if ctx.store != nil {
		if err := ctx.store.CreateService(vsID, opts); err != nil {
			log.Errorf("error while create service : %s", err)
			return err
		}
	}

	var flags int
	for _, flag := range strings.Split(opts.Flags, "|") {
		flags = flags | schedulerFlags[flag]
	}

	if flags != 0 {
		if err := ctx.ipvs.AddServiceWithFlags(
			opts.host.String(),
			opts.Port,
			opts.protocol,
			opts.Method,
			gnl2go.U32ToBinFlags(uint32(flags)),
		); err != nil {
			log.Errorf("error while creating virtual service: %s", err)
			return ErrIpvsSyscallFailed
		}
	} else {
		if err := ctx.ipvs.AddService(
			opts.host.String(),
			opts.Port,
			opts.protocol,
			opts.Method,
		); err != nil {
			log.Errorf("error while creating virtual service: %s", err)
			return ErrIpvsSyscallFailed
		}
	}

	ctx.services[vsID] = &service{options: opts}

	if err := ctx.disco.Expose(vsID, opts.host.String(), opts.Port); err != nil {
		log.Errorf("error while exposing service to Disco: %s", err)
	}

	return nil
}

// CreateService registers a new virtual service with IPVS.
func (ctx *Context) CreateService(vsID string, opts *ServiceOptions) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.createService(vsID, opts)
}

// CreateBackend registers a new backend with a virtual service.
func (ctx *Context) createBackend(vsID, rsID string, opts *BackendOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	p, err := pulse.New(opts.host.String(), opts.Port, opts.Pulse)
	if err != nil {
		return err
	}

	if _, exists := ctx.backends[rsID]; exists {
		return ErrObjectExists
	}

	vs, exists := ctx.services[vsID]

	if !exists {
		return ErrObjectNotFound
	}

	if util.AddrFamily(opts.host) != util.AddrFamily(vs.options.host) {
		return ErrIncompatibleAFs
	}

	log.Infof("creating backend [%s] on %s:%d for virtual service [%s]",
		rsID,
		opts.host,
		opts.Port,
		vsID)

	// create backend to external store
	if ctx.store != nil {
		if err := ctx.store.CreateBackend(vsID, rsID, opts); err != nil {
			log.Errorf("error while create backend : %s", err)
			return err
		}
	}

	if err := ctx.ipvs.AddDestPort(
		vs.options.host.String(),
		vs.options.Port,
		opts.host.String(),
		opts.Port,
		vs.options.protocol,
		int32(opts.Weight),
		opts.methodID,
	); err != nil {
		log.Errorf("error while creating backend: %s", err)
		return ErrIpvsSyscallFailed
	}

	ctx.backends[rsID] = &backend{options: opts, service: vs, monitor: p}

	// Fire off the configured pulse goroutine, attach it to the Context.
	go ctx.backends[rsID].monitor.Loop(pulse.ID{VsID: vsID, RsID: rsID}, ctx.pulseCh, ctx.stopCh)

	return nil
}

// CreateBackend registers a new backend with a virtual service.
func (ctx *Context) CreateBackend(vsID, rsID string, opts *BackendOptions) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.createBackend(vsID, rsID, opts)
}

// UpdateBackend updates the specified backend's weight.
func (ctx *Context) updateBackend(vsID, rsID string, weight int32) (int32, error) {
	rs, exists := ctx.backends[rsID]

	if !exists {
		return 0, ErrObjectNotFound
	}

	log.Infof("updating backend [%s/%s] with weight: %d", vsID, rsID,
		weight)

	if err := ctx.ipvs.UpdateDestPort(
		rs.service.options.host.String(),
		rs.service.options.Port,
		rs.options.host.String(),
		rs.options.Port,
		rs.service.options.protocol,
		weight,
		rs.options.methodID,
	); err != nil {
		log.Errorf("error while updating backend [%s/%s]", vsID, rsID)
		return 0, ErrIpvsSyscallFailed
	}

	var result int32

	// Save the old backend weight and update the current backend weight.
	result, rs.options.Weight = rs.options.Weight, weight

	// Currently the backend options are changing only the weight.
	// The weight value is set to the value requested at the first setting,
	// and the weight value is updated when the pulse fails in the gorb.
	// In kvstore, it seems correct to record the request at the first setting and
	// not reflect the updated weight value.
	//if ctx.store != nil {
	//	ctx.store.UpdateBackend(vsID, rsID, rs.options)
	//}

	return result, nil
}

// UpdateBackend updates the specified backend's weight.
func (ctx *Context) UpdateBackend(vsID, rsID string, weight int32) (int32, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.updateBackend(vsID, rsID, weight)
}

// RemoveService deregisters a virtual service.
func (ctx *Context) removeService(vsID string) (*ServiceOptions, error) {
	vs, exists := ctx.services[vsID]

	if !exists {
		return nil, ErrObjectNotFound
	}

	delete(ctx.services, vsID)

	if ctx.vipInterface != nil && vs.options.delIfAddr == true {
		ifName := ctx.vipInterface.Attrs().Name
		vip := &netlink.Addr{IPNet: &net.IPNet{
			net.ParseIP(vs.options.host.String()), net.IPv4Mask(255, 255, 255, 255)}}
		if err := netlink.AddrDel(ctx.vipInterface, vip); err != nil {
			log.Infof(
				"failed to delete VIP %s to interface '%s' for service [%s]: %s",
				vs.options.host, ifName, vsID, err)
		}
		log.Infof("VIP %s has been deleted from interface '%s'", vs.options.host, ifName)
	}

	log.Infof("removing virtual service [%s] from %s:%d", vsID,
		vs.options.host,
		vs.options.Port)

	if err := ctx.ipvs.DelService(
		vs.options.host.String(),
		vs.options.Port,
		vs.options.protocol,
	); err != nil {
		log.Errorf("error while removing virtual service [%s]", vsID)
		return nil, ErrIpvsSyscallFailed
	}

	// delete service from external store
	if ctx.store != nil {
		if err := ctx.store.RemoveService(vsID); err != nil {
			log.Errorf("error while remove service : %s", err)
		}
	}

	for rsID, backend := range ctx.backends {
		if backend.service != vs {
			continue
		}

		log.Infof("cleaning up now orphaned backend [%s/%s]", vsID, rsID)

		// Stop the pulse goroutine.
		backend.monitor.Stop()

		delete(ctx.backends, rsID)

		// delete backend from external store
		if ctx.store != nil {
			ctx.store.RemoveBackend(rsID)
		}
	}

	// TODO(@kobolog): This will never happen in case of gorb-link.
	if err := ctx.disco.Remove(vsID); err != nil {
		log.Errorf("error while removing service from Disco: %s", err)
	}

	return vs.options, nil
}

// RemoveService deregisters a virtual service.
func (ctx *Context) RemoveService(vsID string) (*ServiceOptions, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.removeService(vsID)
}

// RemoveBackend deregisters a backend.
func (ctx *Context) removeBackend(vsID, rsID string) (*BackendOptions, error) {
	rs, exists := ctx.backends[rsID]

	if !exists {
		return nil, ErrObjectNotFound
	}

	log.Infof("removing backend [%s/%s]", vsID, rsID)

	// delete backend from external store
	if ctx.store != nil {
		if err := ctx.store.RemoveBackend(rsID); err != nil {
			log.Errorf("error while remove backend : %s", err)
		}
	}

	// Stop the pulse goroutine.
	rs.monitor.Stop()

	if err := ctx.ipvs.DelDestPort(
		rs.service.options.host.String(),
		rs.service.options.Port,
		rs.options.host.String(),
		rs.options.Port,
		rs.service.options.protocol,
	); err != nil {
		log.Errorf("error while removing backend [%s/%s]", vsID, rsID)
		return nil, ErrIpvsSyscallFailed
	}

	delete(ctx.backends, rsID)

	return rs.options, nil
}

// RemoveBackend deregisters a backend.
func (ctx *Context) RemoveBackend(vsID, rsID string) (*BackendOptions, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.removeBackend(vsID, rsID)
}

// ListServices returns a list of all registered services.
func (ctx *Context) ListServices() ([]string, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	r := make([]string, 0, len(ctx.services))

	for vsID := range ctx.services {
		r = append(r, vsID)
	}

	return r, nil
}

// ServiceInfo contains information about virtual service options,
// its backends and overall virtual service health.
type ServiceInfo struct {
	Options  *ServiceOptions `json:"options"`
	Health   float64         `json:"health"`
	Backends []string        `json:"backends"`
}

// GetService returns information about a virtual service.
func (ctx *Context) GetService(vsID string) (*ServiceInfo, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	vs, exists := ctx.services[vsID]

	if !exists {
		return nil, ErrObjectNotFound
	}

	result := ServiceInfo{Options: vs.options}

	// This is O(n), can be optimized with reverse backend map.
	for rsID, backend := range ctx.backends {
		if backend.service != vs {
			continue
		}

		result.Backends = append(result.Backends, rsID)
		result.Health += backend.metrics.Health
	}

	if len(result.Backends) == 0 {
		// Service without backends is healthy, albeit useless.
		result.Health = 1.0
	} else {
		result.Health /= float64(len(result.Backends))
	}

	return &result, nil
}

// BackendInfo contains information about backend options and pulse.
type BackendInfo struct {
	Options *BackendOptions `json:"options"`
	Metrics pulse.Metrics   `json:"metrics"`
}

// GetBackend returns information about a backend.
func (ctx *Context) GetBackend(vsID, rsID string) (*BackendInfo, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	rs, exists := ctx.backends[rsID]

	if !exists {
		return nil, ErrObjectNotFound
	}

	return &BackendInfo{rs.options, rs.metrics}, nil
}

// if external kvstore exists, set store to context
func (ctx *Context) SetStore(store *Store) {
	ctx.store = store
}

func (ctx *Context) Synchronize(storeServices map[string]*ServiceOptions, storeBackends map[string]*BackendOptions) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	log.Debugf("============================== SYNC ========================================")
	for k, v := range storeServices {
		log.Debugf("SERVICE[%s]: %s", k, v)
	}
	for k, v := range storeBackends {
		log.Debugf("  BACKEND[%s]: %s", k, v)
	}
	defer log.Debugf("============================================================================")

	// synchronize services with store
	for id, _ := range ctx.services {
		if _, ok := storeServices[id]; !ok {
			ctx.removeService(id)
		}
	}
	for id, storeServiceOptions := range storeServices {
		if service, ok := ctx.services[id]; ok {
			if service.options.CompareStoreOptions(storeServiceOptions) {
				continue
			}
			ctx.removeService(id)
		}
		ctx.createService(id, storeServiceOptions)
	}

	// synchronize backends with store
	for id, backend := range ctx.backends {
		if _, ok := storeBackends[id]; !ok {
			vsID := "(unknown)"
			if len(backend.options.VsID) > 0 {
				vsID = backend.options.VsID
			}
			ctx.removeBackend(vsID, id)
		}
	}
	for id, storeBackendOptions := range storeBackends {
		if backend, ok := ctx.backends[id]; ok {
			if backend.options.CompareStoreOptions(storeBackendOptions) {
				continue
			}
			ctx.removeBackend(storeBackendOptions.VsID, id)
		}
		if err := ctx.createBackend(storeBackendOptions.VsID, id, storeBackendOptions); err != nil {
			log.Warnf("create backend error: %s", err.Error())
		}
	}
}
