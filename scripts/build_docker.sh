#!/bin/bash

set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

VERSION=`git describe --always`

mkdir build
cp ../build/* build/

docker build -t grafana/worldping-gw:$VERSION .
docker tag grafana/worldping-gw:$VERSION grafana/worldping-gw:latest
