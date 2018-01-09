#!/bin/bash
export PYTHONUNBUFFERED=0
while /bin/true; do /autocompile.py $PWD ".go" "make binary" ; done
