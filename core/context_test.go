package core

import (
	"testing"

	"github.com/kobolog/gorb/pulse"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
	"github.com/tehnerd/gnl2go"
	"syscall"
	"github.com/kobolog/gorb/disco"
)

type fakeDisco struct {
	mock.Mock
}

func (d *fakeDisco) Expose(name, host string, port uint16) error {
	args := d.Called(name, host, port)
	return args.Error(0)
}

func (d *fakeDisco) Remove(name string) error {
	args := d.Called(name)
	return args.Error(0)
}

type fakeIpvs struct {
	mock.Mock
}

func (f *fakeIpvs) Init() error {
	args := f.Called()
	return args.Error(0)
}

func (f *fakeIpvs) Exit() {
	f.Called()
}

func (f *fakeIpvs) Flush() error {
	args := f.Called()
	return args.Error(0)
}

func (f *fakeIpvs) AddService(vip string, port uint16, protocol uint16, sched string) error {
	args := f.Called(vip, port, protocol, sched)
	return args.Error(0)
}

func (f *fakeIpvs) AddServiceWithFlags(vip string, port uint16, protocol uint16, sched string, flags []byte) error {
	args := f.Called(vip, port, protocol, sched, flags)
	return args.Error(0)
}

func (f *fakeIpvs) DelService(vip string, port uint16, protocol uint16) error {
	args := f.Called(vip, port, protocol)
	return args.Error(0)
}

func (f *fakeIpvs) AddDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16, weight int32, fwd uint32) error {
	args := f.Called(vip, vport, rip, rport, protocol, weight, fwd)
	return args.Error(0)
}

func (f *fakeIpvs) UpdateDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16, weight int32, fwd uint32) error {
	args := f.Called(vip, vport, rip, rport, protocol, weight, fwd)
	return args.Error(0)

}
func (f *fakeIpvs) DelDestPort(vip string, vport uint16, rip string, rport uint16, protocol uint16) error {
	args := f.Called(vip, vport, rip, rport, protocol)
	return args.Error(0)
}

func newRoutineContext(backends map[string]*backend, ipvs Ipvs) *Context {
	c := newContext(ipvs, &fakeDisco{})
	c.backends = backends
	return c
}

func newContext(ipvs Ipvs, disco disco.Driver) *Context {
	return &Context{
		ipvs:     ipvs,
		services: map[string]*service{},
		backends: make(map[string]*backend),
		pulseCh:  make(chan pulse.Update),
		stopCh:   make(chan struct{}),
		disco: disco,
	}
}

var (
	vsID = "virtualServiceId"
	rsID = "realServerID"
	virtualService = service{options: &ServiceOptions{Port: 80, Host: "localhost", Protocol: "tcp"}}
)

func TestServiceIsCreated(t *testing.T) {
	options := &ServiceOptions{Port: 80, Host: "localhost", Protocol: "tcp", Method: "sh"}
	mockIpvs := &fakeIpvs{}
	mockDisco := &fakeDisco{}
	c := newContext(mockIpvs, mockDisco)

	mockIpvs.On("AddService", "127.0.0.1", uint16(80), uint16(syscall.IPPROTO_TCP), "sh").Return(nil)
	mockDisco.On("Expose", vsID, "127.0.0.1", uint16(80)).Return(nil)

	err := c.createService(vsID, options)
	assert.NoError(t, err)
	mockIpvs.AssertExpectations(t)
	mockDisco.AssertExpectations(t)
}

func TestServiceIsCreatedWithCustomFlags(t *testing.T) {
	options := &ServiceOptions{Port: 80, Host: "localhost", Protocol: "tcp", Method: "sh", Flags: "sh-port|sh-fallback"}
	mockIpvs := &fakeIpvs{}
	mockDisco := &fakeDisco{}
	c := newContext(mockIpvs, mockDisco)

	mockIpvs.On("AddServiceWithFlags", "127.0.0.1", uint16(80), uint16(syscall.IPPROTO_TCP), "sh", gnl2go.U32ToBinFlags(gnl2go.IP_VS_SVC_F_SCHED_SH_FALLBACK | gnl2go.IP_VS_SVC_F_SCHED_SH_PORT)).Return(nil)
	mockDisco.On("Expose", vsID, "127.0.0.1", uint16(80)).Return(nil)

	err := c.createService(vsID, options)
	assert.NoError(t, err)
	mockIpvs.AssertExpectations(t)
	mockDisco.AssertExpectations(t)
}

func TestPulseUpdateSetsBackendWeightToZeroOnStatusDown(t *testing.T) {
	stash := make(map[pulse.ID]int32)
	backends := map[string]*backend{rsID: &backend{service: &virtualService, options: &BackendOptions{Weight:100}}}
	mockIpvs := &fakeIpvs{}

	c := newRoutineContext(backends, mockIpvs)

	mockIpvs.On("UpdateDestPort", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, int32(0), mock.Anything).Return(nil)

	c.processPulseUpdate(stash, pulse.Update{pulse.ID{VsID: vsID, RsID: rsID}, pulse.Metrics{Status: pulse.StatusDown}})
	assert.Equal(t, len(stash), 1)
	assert.Equal(t, stash[pulse.ID{VsID: vsID, RsID: rsID}], int32(100))
	mockIpvs.AssertExpectations(t)
}

func TestPulseUpdateIncreasesBackendWeightRelativeToTheHealthOnStatusUp(t *testing.T) {
	stash := map[pulse.ID]int32{pulse.ID{VsID: vsID, RsID: rsID}: int32(12)}
	backends := map[string]*backend{rsID: &backend{service: &virtualService, options: &BackendOptions{}}}
	mockIpvs := &fakeIpvs{}

	c := newRoutineContext(backends, mockIpvs)

	mockIpvs.On("UpdateDestPort", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, int32(6), mock.Anything).Return(nil)

	c.processPulseUpdate(stash, pulse.Update{pulse.ID{VsID: vsID, RsID: rsID}, pulse.Metrics{Status: pulse.StatusUp, Health:0.5}})
	assert.Equal(t, len(stash), 1)
	assert.Equal(t, stash[pulse.ID{VsID: vsID, RsID: rsID}], int32(12))
	mockIpvs.AssertExpectations(t)
}

func TestPulseUpdateRemovesStashWhenBackendHasFullyRecovered(t *testing.T) {
	stash := map[pulse.ID]int32{pulse.ID{VsID: vsID, RsID: rsID}: int32(12)}
	backends := map[string]*backend{rsID: &backend{service: &virtualService, options: &BackendOptions{}}}
	mockIpvs := &fakeIpvs{}

	c := newRoutineContext(backends, mockIpvs)

	mockIpvs.On("UpdateDestPort", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, int32(12), mock.Anything).Return(nil)

	c.processPulseUpdate(stash, pulse.Update{pulse.ID{VsID: vsID, RsID: rsID}, pulse.Metrics{Status: pulse.StatusUp, Health:1}})
	assert.Empty(t, stash)
	mockIpvs.AssertExpectations(t)
}