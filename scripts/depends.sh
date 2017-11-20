#!/bin/bash
set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

: ${GOPATH:="${HOME}/.go_workspace"}
: ${ORG_PATH:="github.com/raintank"}
: ${REPO_PATH:="${ORG_PATH}/worldping-gw"}

export GOPATH

bundle install

