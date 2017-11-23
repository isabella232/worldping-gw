#!/bin/bash

set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

cd ${DIR}/..

out=$(gofmt -d -s $(find . -name '*.go' | grep -v vendor | grep -v _gen.go))
if [ "$out" != "" ]; then
	echo "$out"
	echo
	echo "You might want to run something like 'find . -name '*.go' | xargs gofmt -w -s'"
	exit 2
fi


./scripts/vendor_health.sh || exit 2
go vet $(go list ./... | grep -v /vendor/) || exit 2
go test ./...