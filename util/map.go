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
	"reflect"
)

// DynamicMap are arbitrary options passed directly to the driver.
type DynamicMap map[string]interface{}

// Get returns a typed option or a default value if the option is not set.
func (do DynamicMap) Get(key string, d interface{}) interface{} {
	if v, exists := do[key]; !exists {
		return d
	} else if vt, dt := reflect.TypeOf(v), reflect.TypeOf(d); vt.ConvertibleTo(dt) {
		return v
	} else {
		return d
	}
}
