#!/bin/sh
set -e
tmpFile=$(mktemp)
CGO_ENABLED=0 go build -o "$tmpFile" app/*.go
exec "$tmpFile" "$@"