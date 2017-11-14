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

const (
	virtualServiceId = "vsID"
)

func TestServiceIsCreated(t *testing.T) {
	options := &ServiceOptions{Port: 80, Host: "localhost", Protocol: "tcp", Method: "sh"}
	mockIpvs := &fakeIpvs{}
	mockDisco := &fakeDisco{}
	c := newContext(mockIpvs, mockDisco)

	mockIpvs.On("AddService", "127.0.0.1", uint16(80), uint16(syscall.IPPROTO_TCP), "sh").Return(nil)
	mockDisco.On("Expose", virtualServiceId, "127.0.0.1", uint16(80)).Return(nil)

	err := c.createService(virtualServiceId, options)
	assert.NoError(t, err)
	mockIpvs.AssertExpectations(t)
	mockDisco.AssertExpectations(t)
}

func TestServiceIsCreatedWithCustomFlags(t *testing.T) {
	options := &ServiceOptions{Port: 80, Host: "localhost", Protocol: "tcp", Method: "sh", Flags: "sh-port|sh-fallback"}
	mockIpvs := &fakeIpvs{}
	mockDisco := &fakeDisco{}
	c := newContext(mockIpvs, mockDisco)

	mockIpvs.On("AddServiceWithFlags", "127.0.0.1", uint16(80), uint16(syscall.IPPROTO_TCP), "sh", gnl2go.U32ToBinFlags(gnl2go.IP_VS_SVC_F_SCHED_SH_FALLBACK|gnl2go.IP_VS_SVC_F_SCHED_SH_PORT)).Return(nil)
	mockDisco.On("Expose", virtualServiceId, "127.0.0.1", uint16(80)).Return(nil)

	err := c.createService(virtualServiceId, options)
	assert.NoError(t, err)
	mockIpvs.AssertExpectations(t)
	mockDisco.AssertExpectations(t)
}
