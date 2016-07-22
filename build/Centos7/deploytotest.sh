# Create a test environment, run tests, and optionally tear down env if tests all pass.

pushd $( dirname "${BASH_SOURCE[0]}" )
export BuildDir=`pwd`
echo BuildDir=$BuildDir
popd
export DeployDir=$BuildDir/../../deploy/Compose
echo DeployDir=$DeployDir

source $BuildDir/env.sh  # Load build configuration.

# Set deploy configuration for testing.
export SafeHarborPublicHostname=52.39.70.179
export RandomString=alkejfa4ak0s3
export DataVolMountPoint=/home/centos/safeharbordata  # this gets mapped to the container /safeharbor/data directory.
export registryUser=safeharbor
export registryPassword=gksspie8a
export postgresPassword=4word2day
export SafeHarborPort=6000
export RegistryPort=5000
export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver

# Deploy and test.
$BuildDir/createdeployenv.sh	# Create a test env.
$DeployDir/deploy.sh			# Deploy to the test env.
$BuildDir/test.sh				# Run tests.
$DeployDir/undeploy.sh			# Destroy test env.
