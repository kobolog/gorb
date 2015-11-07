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
	"errors"
	"strings"
	"time"

	"github.com/kobolog/gorb/util"
)

// Possible validation errors.
var (
	ErrMissingHTTPPulsePath = errors.New("path for HTTP pulse is missing")
	ErrUnknownPulseType     = errors.New("specified pulse type is unknown")
	ErrInvalidPulseInterval = errors.New("pulse interval must be positive")
)

// Options contain Pulse configuration.
type Options struct {
	Type     string `json:"type"`
	Interval string `json:"interval"`
	Path     string `json:"path,omitempty"`

	interval time.Duration
}

// Validate fills missing fields and validates Pulse configuration.
func (o *Options) Validate() error {
	if len(o.Type) == 0 {
		o.Type, o.Interval = "tcp", "1m"
	}

	o.Type = strings.ToLower(o.Type)

	switch o.Type {
	case "tcp":
		// Nothing special for this Pulse type.
	case "http":
		if len(o.Path) == 0 {
			return ErrMissingHTTPPulsePath
		}
	default:
		return ErrUnknownPulseType
	}

	var err error

	if o.interval, err = util.ParseInterval(o.Interval); err != nil {
		return err
	} else if o.interval <= 0 {
		return ErrInvalidPulseInterval
	}

	return nil
}
