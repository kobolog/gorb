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

// StatusType represents the backend's pulse status code.
type StatusType int

const (
	// StatusUp means the backend is up and healthy.
	StatusUp StatusType = iota
	// StatusDown means the backend is not responding to pulse.
	StatusDown
)

func (status StatusType) String() string {
	switch status {
	case StatusUp:
		return "Up"
	case StatusDown:
		return "Down"
	}

	return "Unknown"
}

// ID is a (vsID, rsID) tuple used in pulse notifications.
type ID struct {
	VsID, RsID string
}

// Status is a pulse notification.
type Status struct {
	Source ID
	Result StatusType
}
