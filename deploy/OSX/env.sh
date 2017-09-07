#!/bin/sh

# Env config for OSX.
# Edit these values for your deployment:
export DataVolMountPoint=/Transient/safeharbor/data  # this gets mapped to the container data directory.
export registryUser=safeharbor
export SafeHarborServerHost=54.71.85.235
export SafeHarborPort=6000
export RegistryPort=5000

# Do not change:
export PROJECTROOT=$( dirname "${BASH_SOURCE[0]}" )/../..
. ../../build/common/env.sh
export ScaledMarketsRegistryNamespace=scaledmarkets
#export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
export ENV_CONFIGURED=yes
