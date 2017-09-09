#!/bin/sh

# Env config for OSX.
# Edit these values for your deployment:
# Note: The DataVolMountPoint must be "shared" with the docker daemon. Use
# the docker app configuration tool to do this.
export DataVolMountPoint=/Transient/safeharbor/data  # this gets mapped to the container data directory.
export registryUser=safeharbor
export SafeHarborServerHost=127.0.0.1
export SafeHarborPort=6000
export RegistryPort=5000

# Do not change:
export PROJECTROOT=$( cd $( dirname ${BASH_SOURCE[0]} ) && pwd )/../..
. $PROJECTROOT/build/common/env.sh
#export ScaledMarketsRegistryNamespace=scaledmarkets
#export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
#export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
export TARGET_ENV_CONFIGURED=yes
