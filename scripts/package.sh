#!/bin/bash
set -x
BASE=$(dirname $0)
CODE_DIR=$(readlink -e "$BASE/../")

sudo apt-get install rpm

BUILD_ROOT=$CODE_DIR/build

ARCH="$(uname -m)"
VERSION=$(git describe --long --always)

## ubuntu 14.04
BUILD=${BUILD_ROOT}/upstart

PACKAGE_NAME="${BUILD}/worldping-gw-${VERSION}_${ARCH}.deb"

mkdir -p ${BUILD}/usr/bin
mkdir -p ${BUILD}/etc/init
mkdir -p ${BUILD}/etc/raintank

cp ${BASE}/config/gw.ini ${BUILD}/etc/worldping/
cp ${BUILD_ROOT}/worldping-gw ${BUILD}/usr/bin/

fpm -s dir -t deb \
  -v ${VERSION} -n worldping-gw -a ${ARCH} --description "HTTP gateway service for worldping" \
  --deb-upstart ${BASE}/config/upstart/worldping-gw \
  -C ${BUILD} -p ${PACKAGE_NAME} .

## ubuntu 16.04, Debian 8, CentOS 7
BUILD=${BUILD_ROOT}/systemd
PACKAGE_NAME="${BUILD}/worldping-gw-${VERSION}_${ARCH}.deb"
mkdir -p ${BUILD}/usr/bin
mkdir -p ${BUILD}/lib/systemd/system/
mkdir -p ${BUILD}/etc/worldping
mkdir -p ${BUILD}/var/run/worldping

cp ${BASE}/config/tsdb.ini ${BUILD}/etc/worldping/
cp ${BUILD_ROOT}/worldping-gw ${BUILD}/usr/bin/
cp ${BASE}/config/systemd/worldping-gw.service $BUILD/lib/systemd/system

fpm -s dir -t deb \
  -v ${VERSION} -n worldping-gw -a ${ARCH} --description "HTTP gateway service for worldping" \
  --config-files /etc/worldping/ \
  -m "Raintank Inc. <hello@raintank.io>" --vendor "raintank.io" \
  --license "Apache2.0" -C ${BUILD} -p ${PACKAGE_NAME} .

BUILD=${BUILD_ROOT}/systemd-centos7

mkdir -p ${BUILD}/usr/sbin
mkdir -p ${BUILD}/lib/systemd/system/
mkdir -p ${BUILD}/etc/worldping
mkdir -p ${BUILD}/var/run/worldping

cp ${BASE}/config/gw.ini ${BUILD}/etc/worldping/
cp ${BUILD_ROOT}/worldping-gw ${BUILD}/usr/bin/
cp ${BASE}/config/systemd/worldping-gw.service $BUILD/lib/systemd/system

PACKAGE_NAME="${BUILD}/worldping-gw-${VERSION}.el7.${ARCH}.rpm"

fpm -s dir -t rpm \
  -v ${VERSION} -n worldping-gw -a ${ARCH} --description "HTTP gateway service for worldping" \
  --config-files /etc/worldping/ \
  -m "Raintank Inc. <hello@raintank.io>" --vendor "raintank.io" \
  --license "Apache2.0" -C ${BUILD} -p ${PACKAGE_NAME} .


