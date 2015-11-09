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
	"time"
)

// Metrics contain statistical information about backend's Pulse.
type Metrics struct {
	Status StatusType    `json:"status"`
	Health float64       `json:"health"`
	Uptime time.Duration `json:"uptime"`

	// Historical information for statistics calculation.
	lastTs time.Time
	record []StatusType
}

// NewMetrics creates a new instance of metrics.
func NewMetrics() *Metrics {
	return &Metrics{Status: StatusUp, Health: 1, Uptime: 0, lastTs: time.Now()}
}

// Update updates metrics based on Pulse status message.
func (m *Metrics) Update(status StatusType) Metrics {
	m.Status = status
	m.Health = 0
	m.record = append(m.record, status)

	if len(m.record) > 100 {
		m.record = m.record[1:]
	}

	for _, result := range m.record {
		m.Health += float64(result)
	}

	m.Health = 1.0 - m.Health/float64(len(m.record))

	if ts := time.Now(); m.Status != StatusUp {
		m.Uptime, m.lastTs = 0, ts
	} else {
		m.Uptime, m.lastTs = m.Uptime+ts.Sub(m.lastTs)/time.Second, ts
	}

	return *m
}
