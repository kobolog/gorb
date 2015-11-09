#!/usr/bin/env bash

OUTPUT=>(echo >> coverage.txt)

set -e

for pkg in $@; do
    go test -coverprofile=$OUTPUT -covermode=atomic ./$pkg
done
