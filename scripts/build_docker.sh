#!/bin/bash

set -x
# Find the directory we exist within
DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd ${DIR}

VERSION=`git describe --always`

mkdir build
cp ../build/* build/

docker build -t raintank/worldping-gw:$VERSION .
docker tag raintank/worldping-gw:$VERSION raintank/worldping-gw:latest
