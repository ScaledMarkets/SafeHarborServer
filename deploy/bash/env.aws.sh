#!/bin/sh

# Env config for Centos.
# Edit these values for your deployment:
export DataVolMountPoint=/safeharbor/data  # this gets mapped to the container data directory.
export registryUser=safeharbor
export SafeHarborServerHost=54.71.85.235
export SafeHarborPort=6000
export RegistryPort=5000

export TwistlockDir=/twistlock

# Do not change:
pushd $( dirname "${BASH_SOURCE[0]}" )/../..
export PROJECTROOT=`pwd`
echo "PROJECTROOT=$PROJECTROOT"
popd
. $PROJECTROOT/build/common/env.sh
export ScaledMarketsRegistryNamespace=scaledmarkets
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
export TARGET_ENV_CONFIGURED=yes
