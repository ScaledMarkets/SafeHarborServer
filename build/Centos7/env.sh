# Environment configuration used by all build scripts.

export PROJECTROOT=$( dirname "${BASH_SOURCE[0]}" )/../..
export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
export SSHTestKeyPath=$PROJECTROOT/deploy/AWS/SafeHarborServer.pem
export DataVolMountPoint=/home/centos/safeharbordata  # this gets mapped to the container data directory.
