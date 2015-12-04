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

package disco

import (
	"github.com/kobolog/gorb/util"
)

// Driver provides the actual implementation for the Discovery.
type Driver interface {
	Expose(name, host string, port uint16) error
	Remove(name string) error
}

// Options contain Discovery configuration.
type Options struct {
	Type string
	Args util.DynamicMap
}

// New creates a new Discovery from the provided options.
func New(opts *Options) (Driver, error) {
	return newConsulDriver(opts.Args)
}
