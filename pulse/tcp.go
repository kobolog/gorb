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
	"time"

	log "github.com/sirupsen/logrus"
)

type tcpPulse struct {
	Driver

	endpoint string
	dialer   net.Dialer
}

func newTCPDriver(host string, port uint16, opts *Options) Driver {
	return &tcpPulse{
		endpoint: fmt.Sprintf("%s:%d", host, port),
		dialer:   net.Dialer{DualStack: true, Timeout: 5 * time.Second},
	}
}

func (p *tcpPulse) Check() StatusType {
	if socket, err := p.dialer.Dial("tcp", p.endpoint); err != nil {
		log.Errorf("unable to connect to %s", p.endpoint)
	} else {
		socket.Close()
		return StatusUp
	}

	return StatusDown
}
