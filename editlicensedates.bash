#!/usr/bin/env bash

# Copyright (c) 2015 Eric Lagergren
# This is public domain.

set -euo pipefail

shopt -s globstar nullglob extglob
for f in **/*.@(go); do
	if [[ -f $f ]]; then
		sed -i 's/Copyright (C) 2014 /Copyright (c) 2014-2015 /g' "$f"
	fi
done