

pushd $( dirname "${BASH_SOURCE[0]}" )/../..
export PROJECTROOT=`pwd`
echo "PROJECTROOT=$PROJECTROOT"
popd

export ScaledMarketsCredentialsDir=~/Documents/ScaledMarkets/Credentials
export SafeHarborCredentialDir=$PROJECTROOT/Credentials

export ScaledMarketsRegistryNamespace=scaledmarkets
#export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver

export RegistryAuthDir=$DataVolMountPoint/registryauth
export DataDir=$DataVolMountPoint/registrydata
export CertDir=$DataVolMountPoint/registrycerts
export ClairDir=$DataVolMountPoint/clair
export PostgresDir=$DataVolMountPoint/postgres
export RedisConfigDir=$DataVolMountPoint/redis/conf
export RedisDataDir=$DataVolMountPoint/redis/data
