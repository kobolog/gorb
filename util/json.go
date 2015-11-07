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
	"encoding/json"
)

// JSONOptions configure JSON marshalling behavior.
type JSONOptions struct {
	Indent bool
}

// MustMarshal will try to convert the given object to JSON or panic if there's an error.
func MustMarshal(object interface{}, options JSONOptions) []byte {
	output, err := json.Marshal(&object)

	if err != nil {
		panic(err)
	}

	switch {
	case options.Indent:
		buffer := bytes.Buffer{}

		// TODO(@kobolog): Expose indentation options via JSONOptions.
		json.Indent(&buffer, output, "", "\t")

		return buffer.Bytes()

	default:
		return output
	}
}
