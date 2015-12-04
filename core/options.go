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
	"net"
	"strings"
	"syscall"

	"github.com/kobolog/gorb/pulse"

	"github.com/tehnerd/gnl2go"
)

// Possible validation errors.
var (
	ErrMissingEndpoint = errors.New("endpoint information is missing")
	ErrUnknownMethod   = errors.New("specified forwarding method is unknown")
	ErrUnknownProtocol = errors.New("specified protocol is unknown")
)

// ContextOptions configure Context behavior.
type ContextOptions struct {
	Disco     string
	Endpoints []net.IP
	Flush     bool
}

// ServiceOptions describe a virtual service.
type ServiceOptions struct {
	Host       string `json:"host"`
	Port       uint16 `json:"port"`
	Protocol   string `json:"protocol"`
	Method     string `json:"method"`
	Persistent bool   `json:"persistent"`

	// Host string resolved to an IP, including DNS lookup.
	host net.IP

	// Protocol string converted to a protocol number.
	protocol uint16
}

// Validate fills missing fields and validates virtual service configuration.
func (o *ServiceOptions) Validate(defaultHost net.IP) error {
	if o.Port == 0 {
		return ErrMissingEndpoint
	}

	if len(o.Host) != 0 {
		if addr, err := net.ResolveIPAddr("ip", o.Host); err == nil {
			o.host = addr.IP
		} else {
			return err
		}
	} else if defaultHost != nil {
		o.host = defaultHost
	} else {
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
		return ErrUnknownProtocol
	}

	if len(o.Method) == 0 {
		// WRR since Pulse will dynamically reweight backends.
		o.Method = "wrr"
	}

	return nil
}

// BackendOptions describe a virtual service backend.
type BackendOptions struct {
	Host   string         `json:"host"`
	Port   uint16         `json:"port"`
	Weight int32          `json:"weight"`
	Method string         `json:"method"`
	Pulse  *pulse.Options `json:"pulse"`

	// Host string resolved to an IP, including DNS lookup.
	host net.IP

	// Forwarding method string converted to a forwarding method number.
	methodID uint32
}

// Validate fills missing fields and validates backend configuration.
func (o *BackendOptions) Validate() error {
	if len(o.Host) == 0 || o.Port == 0 {
		return ErrMissingEndpoint
	}

	if addr, err := net.ResolveIPAddr("ip", o.Host); err == nil {
		o.host = addr.IP
	} else {
		return err
	}

	if o.Weight <= 0 {
		o.Weight = 100
	}

	if len(o.Method) == 0 {
		o.Method = "nat"
	}

	o.Method = strings.ToLower(o.Method)

	switch o.Method {
	case "nat":
		o.methodID = gnl2go.IPVS_MASQUERADING
	case "tunnel", "ipip":
		o.methodID = gnl2go.IPVS_TUNNELING
	default:
		return ErrUnknownMethod
	}

	if o.Pulse == nil {
		// It doesn't make much sense to have a backend with no Pulse.
		o.Pulse = &pulse.Options{}
	}

	return nil
}
