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
	"flag"
	"net/http"
	"os"

	"github.com/kobolog/gorb/core"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	listen = flag.String("l", ":8080", "endpoint to listen for HTTP connection")
	flush  = flag.Bool("f", false, "flush IPVS tables on start")
)

func main() {
	if os.Geteuid() != 0 {
		log.Fatalf("this program has to be run with root priveleges")
	}

	// TODO(@kobolog): Look into replacing with getopt for long options.
	flag.Parse()

	ctx, err := core.NewContext(core.ContextOptions{Flush: *flush})

	if err != nil {
		log.Fatalf("error while initializing server context: %s", err)
	}

	r := mux.NewRouter()

	r.Handle("/service/{vsID}", serviceCreateHandler{ctx}).Methods("PUT")
	r.Handle("/service/{vsID}/{rsID}", backendCreateHandler{ctx}).Methods("PUT")
	r.Handle("/service/{vsID}/{rsID}", backendUpdateHandler{ctx}).Methods("PATCH")
	r.Handle("/service/{vsID}", serviceRemoveHandler{ctx}).Methods("DELETE")
	r.Handle("/service/{vsID}/{rsID}", backendRemoveHandler{ctx}).Methods("DELETE")
	r.Handle("/service/{vsID}", serviceStatusHandler{ctx}).Methods("GET")
	r.Handle("/service/{vsID}/{rsID}", backendStatusHandler{ctx}).Methods("GET")

	// While it's not strictly required, close IPVS socket explicitly.
	defer ctx.Close()

	log.Infof("setting up HTTP server on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, r))
}
