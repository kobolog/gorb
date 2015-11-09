#!/usr/bin/env bash

set -e

echo "" > coverage.txt

for pkg in $@; do
    go test -coverprofile=profile.out -covermode=atomic ./$pkg

    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm  profile.out
    fi
done
