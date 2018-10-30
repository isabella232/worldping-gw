#!/bin/bash

set -xe

SCRIPTS_DIR=$(dirname ${BASH_SOURCE[0]})
SOURCE_DIR=$SCRIPTS_DIR/..
cd $SOURCE_DIR

go test -v -race ./...
go vet ./...
