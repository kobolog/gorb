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
	"reflect"
	"strings"
	"time"

	"github.com/kobolog/gorb/util"

	log "github.com/sirupsen/logrus"
)

// Possible validation errors.
var (
	ErrUnknownPulseType     = errors.New("specified pulse type is unknown")
	ErrInvalidPulseInterval = errors.New("pulse interval must be positive")
)

// DriverOptions are arbitrary options passed directly to the driver.
type DriverOptions map[string]interface{}

// Options contain Pulse configuration.
type Options struct {
	Type     string        `json:"type"`
	Interval string        `json:"interval"`
	Args     DriverOptions `json:"args"`

	interval time.Duration
}

// Get returns a typed option or a default value if the option is not set.
func (do DriverOptions) Get(key string, d interface{}) interface{} {
	if v, exists := do[key]; !exists {
		return d
	} else if vt, dt := reflect.TypeOf(v), reflect.TypeOf(d); vt.ConvertibleTo(dt) {
		return v
	} else {
		log.Warnf("got %s instead of %s for option %s", vt, dt, key)
		return d
	}
}

// Validate fills missing fields and validates Pulse configuration.
func (o *Options) Validate() error {
	if len(o.Type) == 0 {
		// TCP is a safe guess: the majority of services are TCP-based.
		o.Type = "tcp"
	}

	if len(o.Interval) == 0 {
		o.Interval = "1m"
	}

	o.Type = strings.ToLower(o.Type)

	if fn := get[o.Type]; fn == nil {
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
