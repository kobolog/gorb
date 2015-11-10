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

package util

import (
	"bytes"
	"net"
	"syscall"
)

// Possible address families.
const (
	IPv4 = syscall.AF_INET
	IPv6 = syscall.AF_INET6
)

// All IP addresses are stored as 16-byte slices internally and
// there's no built-in function to tell them apart.
const (
	v4Length = 4
	v6Length = 16
)

var v4Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}

// AddrFamily returns the address family of an IP address.
func AddrFamily(ip net.IP) int {
	if len(ip) == v4Length {
		return IPv4
	}

	if len(ip) == v6Length && bytes.HasPrefix(ip, v4Prefix) {
		return IPv4
	}

	return IPv6
}

// InterfaceIPs returns a slice of interface IP addresses.
func InterfaceIPs(device string) (ips []net.IP, _ error) {
	var networks []net.Addr

	if iface, err := net.InterfaceByName(device); err != nil {
		return nil, err
	} else if networks, err = iface.Addrs(); err != nil {
		return nil, err
	}

	for _, network := range networks {
		if ipNet, castable := network.(*net.IPNet); castable {
			ips = append(ips, ipNet.IP)
		}
	}

	// Note that on non-IP interfaces it will be an empty slice
	// and no error indicator. Maybe that's not super-perfect.
	return ips, nil
}
