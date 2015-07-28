#!/usr/bin/env bash

# Copyright (c) 2015 Eric Lagergren
# This is public domain.

# No set -e because we expect some installs to fail.
set -uo pipefail

: > build.log

for d in *; do
	if [ -d "${d}" ]; then
		
		cd "${d}"
		go generate >> ../build.log 2>&1
		go install -a -v -x -gcflags '-m' >> ../build.log 2>&1
		cd ../
		
	fi
done