# Variables used by all deployment scripts.

export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
export DataVolMountPoint=/home/centos/safeharbordata  # this gets mapped to the container data directory.
export registryUser=safeharbor
export registryPassword=gksspie8a
