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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	errInvalidIntervalFormat = errors.New("invalid interval format")
	reInterval               *regexp.Regexp

	intervals = map[string]time.Duration{
		"s":       time.Second,
		"sec":     time.Second,
		"seconds": time.Second,
		"m":       time.Minute,
		"min":     time.Minute,
		"minutes": time.Minute,
		"h":       time.Hour,
		"hours":   time.Hour,
	}
)

// ParseInterval parses an interval string and returns the corresponding duration.
func ParseInterval(s string) (time.Duration, error) {
	if m := reInterval.FindStringSubmatch(strings.TrimSpace(s)); len(m) != 0 {
		value, _ := strconv.ParseInt(m[1], 10, 32)
		duration := intervals[strings.ToLower(m[2])]

		return duration * time.Duration(value), nil
	}

	return 0, errInvalidIntervalFormat
}

func init() {
	intervalNames := make([]string, 0, len(intervals))

	for name := range intervals {
		intervalNames = append(intervalNames, name)
	}

	reInterval = regexp.MustCompile(fmt.Sprintf("(?i)^([+-]?[0-9]+)(%s)$",
		strings.Join(intervalNames, "|")))
}
