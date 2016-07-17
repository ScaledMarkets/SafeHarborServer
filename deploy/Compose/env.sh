# Edit these values for your deployment:
export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export DataVolMountPoint=/home/centos/safeharbordata  # this gets mapped to the container data directory.
export registryUser=safeharbor
export registryPassword=gksspie8a
export postgresPassword=4word2day
export SafeHarborPort=6000
export RegistryPort=5000

# Do not change:
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver
