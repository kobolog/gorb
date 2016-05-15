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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/kobolog/gorb/core"
	"github.com/kobolog/gorb/pulse"
	"github.com/kobolog/gorb/util"

	log "github.com/Sirupsen/logrus"
	gdc "github.com/fsouza/go-dockerclient"
)

var (
	device = flag.String("i", "eth0", "default interface to bind public ports on")
	debug  = flag.Bool("v", false, "verbose output")
	remote = flag.String("r", "localhost:4672", "GORB remote endpoint")

	// Default addresses to bind public ports on.
	hostIPs []net.IP

	// HTTP client. Required because of PUT and DELETE GORB requests.
	client http.Client

	// Caches information about already exposed virtual services.
	exposed = make(map[string]struct{})
)

func roundtrip(rqst *http.Request, eh map[int]func() error) error {
	var r *http.Response

	r, err := client.Do(rqst)
	if err == nil {
		defer r.Body.Close()
	} else {
		return fmt.Errorf("GORB HTTP communication error: %s", err)
	}

	if r.StatusCode == http.StatusOK {
		return nil
	} else if h, exists := eh[r.StatusCode]; exists {
		return h()
	}

	// Some weird thing happened.
	var content interface{}

	if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
		return fmt.Errorf("got unknown error from %s", rqst.URL)
	}

	return fmt.Errorf("got unknown error from %s: %v", rqst.URL, content)
}

func createService(vs string, n int64, proto string) error {
	log.Infof("creating service [%s] on port %d/%s", vs, n, proto)

	opts := core.ServiceOptions{Port: uint16(n), Protocol: proto}
	data := bytes.NewBuffer(util.MustMarshal(opts, util.JSONOptions{}))

	rqst, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://%s", path.Join(*remote, "service", vs)),
		data)

	return roundtrip(rqst, map[int]func() error{
		http.StatusConflict: func() error {
			exposed[vs] = struct{}{}
			return nil // not actually an error.
		}})
}

func createBackend(vs, rs string, b gdc.APIPort) error {
	log.Infof("creating [%s] with %s:%d -> %d", path.Join(vs, rs), b.IP,
		b.PublicPort, b.PrivatePort)

	if _, exists := exposed[vs]; !exists {
		r, err := http.Get(
			fmt.Sprintf("http://%s", path.Join(*remote, "service", vs)))
		if err != nil {
			return err
		}

		if r.StatusCode == 200 {
			// Service was pre-exposed earlier.
			exposed[vs] = struct{}{}
		} else if err := createService(vs, b.PrivatePort, b.Type); err != nil {
			return err
		}
	}

	opts := core.BackendOptions{Host: b.IP, Port: uint16(b.PublicPort)}

	if b.Type == "udp" {
		// Disable pulse for UDP backends.
		opts.Pulse = &pulse.Options{Type: "none"}
	}

	data := bytes.NewBuffer(util.MustMarshal(opts, util.JSONOptions{}))

	rqst, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://%s", path.Join(*remote, "service", vs, rs)),
		data)

	return roundtrip(rqst, map[int]func() error{
		http.StatusConflict: func() error {
			return fmt.Errorf("backend [%s] was already created", path.Join(vs, rs))
		},
		http.StatusNotFound: func() error {
			return fmt.Errorf("service parent [%s] cannot be found", vs)
		}})
}

func removeBackend(vs, rs string, b gdc.APIPort) error {
	log.Infof("removing [%s] with %s:%d -> %d", path.Join(vs, rs), b.IP,
		b.PublicPort, b.PrivatePort)

	rqst, _ := http.NewRequest(
		"DELETE",
		fmt.Sprintf("http://%s", path.Join(*remote, "service", vs, rs)),
		nil)

	return roundtrip(rqst, map[int]func() error{
		http.StatusNotFound: func() error {
			return fmt.Errorf("backend [%s] cannot be found", path.Join(vs, rs))
		}})
}

type portAction func(vs, rs string, binding gdc.APIPort) error

func invokeFunc(vs, rs string, ports []gdc.APIPort, fn portAction) []error {
	n := 0
	e := []error{}

	for _, binding := range ports {
		if binding.PrivatePort == 0 || binding.PublicPort == 0 {
			// There is a bug in GDC where unexported ports will have PrivatePort
			// set to zero, instead of PublicPort. So checking both, just in case.
			continue
		}

		if binding.IP == "0.0.0.0" {
			// Rewrite "catch-all" host to a real host's IP address.
			binding.IP = hostIPs[0].String()
		}

		// Mangle the VS name.
		vsID := fmt.Sprintf("%s-%d-%s", strings.Map(func(r rune) rune {
			switch r {
			case '/', ':':
				return '-'
			default:
				return r
			}
		}, vs), binding.PrivatePort, binding.Type)

		// Mangle the RS name.
		rsID := fmt.Sprintf("%s-%d-%s", strings.Map(func(r rune) rune {
			switch r {
			case '/', ':':
				return '-'
			default:
				return r
			}
		}, rs), binding.PublicPort, binding.Type)

		// There must be leading slash in the swarm event stream for node names that needs
		// to be trimmed now that it has been munged
		rsID = strings.Trim(rsID, "-")

		if err := fn(vsID, rsID, binding); err == nil {
			n++
		} else {
			e = append(e, err)
		}
	}

	if n == 0 {
		log.Warnf("no public ports were processed for [%s]", path.Join(vs, rs))
	}

	return e
}

func main() {
	// Called first to interrupt bootstrap and display usage if the user passed -h.
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Infof("starting GORB Docker Link Daemon v0.1")

	if ips, err := util.InterfaceIPs(*device); err != nil {
		log.Fatalf("error while obtaining interface addresses: %s", err)
	} else {
		// Fuck Go scoping rules, yay!
		hostIPs = ips
	}

	actions := map[string]portAction{
		"start": createBackend,
		"kill":  removeBackend,
	}

	c, err := gdc.NewClientFromEnv()
	if err != nil {
		log.Fatalf("error while instantiating Docker client: %s", err)
	}

	log.Infof("listening on event feed at %s", c.Endpoint())

	r, err := c.ListContainers(gdc.ListContainersOptions{})
	if err != nil {
		log.Fatalf("error while listing containers: %s", err)
	}

	if len(r) != 0 {
		log.Infof("bootstrapping with existing containers")

		e := []error{}

		for _, o := range r {
			e = append(e, invokeFunc(o.Image, o.Names[0], o.Ports, createBackend)...)
		}

		if len(e) != 0 {
			log.Warnf("errors while exposing existing containers: %s", e)
		}
	}

	l := make(chan *gdc.APIEvents)
	defer close(l)

	// Client has built-in retries and reconnect support, so nothing else we can do.
	c.AddEventListener(l)

	for event := range l {
		log.Debugf("received event [%s] for container %s", event.Status, event.ID)

		fn, known := actions[event.Status]
		if !known {
			continue
		}

		i, err := c.InspectContainer(event.ID)
		if err != nil {
			log.Errorf("error while inspecting container %s: %s", event.ID, err)
			continue
		}

		b := i.NetworkSettings.PortMappingAPI()
		if len(b) == 0 {
			log.Infof("container %s has no exported ports", event.ID)
			continue
		}

		if e := invokeFunc(i.Config.Image, i.Name, b, fn); len(e) != 0 {
			log.Errorf("error(s) while processing container %s: %s", event.ID, e)
		}
	}

	log.Infof("event feed has been closed, terminating the Daemon")
}
