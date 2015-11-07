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
	"strings"
	"syscall"

	"github.com/kobolog/gorb/pulse"

	"github.com/tehnerd/gnl2go"
)

// Possible validation errors.
var (
	ErrMissingEndpoint = errors.New("endpoint information is missing")
	ErrUnknownMethod   = errors.New("specified forwarding method is unknown")
)

// ContextOptions configure Context behavior.
type ContextOptions struct {
	Flush bool
}

// ServiceOptions describe a virtual service.
type ServiceOptions struct {
	Address    string `json:"address"`
	Port       uint16 `json:"port"`
	Protocol   string `json:"protocol"`
	Method     string `json:"method"`
	Persistent bool   `json:"persistent"`

	// Protocol string converted to a protocol number.
	protocol uint16
}

// Validate fills missing fields and validates virtual service configuration.
func (o *ServiceOptions) Validate() error {
	if len(o.Address) == 0 || o.Port == 0 {
		return ErrMissingEndpoint
	}

	if len(o.Protocol) == 0 {
		o.Protocol = "tcp"
	}

	o.Protocol = strings.ToLower(o.Protocol)

	switch o.Protocol {
	case "tcp":
		o.protocol = syscall.IPPROTO_TCP
	case "udp":
		o.protocol = syscall.IPPROTO_UDP
	default:
		o.protocol = syscall.IPPROTO_TCP
	}

	if len(o.Method) == 0 {
		o.Method = "RR"
	}

	return nil
}

// BackendOptions describe a virtual service backend.
type BackendOptions struct {
	Address string         `json:"address"`
	Port    uint16         `json:"port"`
	Weight  uint32         `json:"weight"`
	Method  string         `json:"method"`
	Pulse   *pulse.Options `json:"pulse"`

	// Forwarding method string converted to a forwarding method number.
	method uint32
}

// Validate fills missing fields and validates backend configuration.
func (o *BackendOptions) Validate() error {
	if len(o.Address) == 0 || o.Port == 0 {
		return ErrMissingEndpoint
	}

	if o.Weight == 0 {
		o.Weight = 100
	}

	if len(o.Method) == 0 {
		o.Method = "nat"
	}

	o.Method = strings.ToLower(o.Method)

	switch o.Method {
	case "nat":
		o.method = gnl2go.IPVS_MASQUERADING
	case "tunnel", "ipip":
		o.method = gnl2go.IPVS_TUNNELING
	default:
		o.method = gnl2go.IPVS_MASQUERADING
	}

	if o.Pulse == nil {
		o.Pulse = &pulse.Options{}
	}

	return o.Pulse.Validate()
}
