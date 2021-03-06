#!/bin/sh
# Create a test environment and run tests.
# Run this on the machine that hosts the containers under test.
# Arguments:
#	$1 - Public IP address or DNS name of the test server.
#
# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the build env shell.

pushd $( dirname "${BASH_SOURCE[0]}" )
export BuildDir=`pwd`
echo BuildDir=$BuildDir
popd
pushd $BuildDir/../../deploy/Compose
export DeployDir=`pwd`
popd
echo DeployDir=$DeployDir

source $BuildDir/env.sh  # Load build configuration.

# Set deploy configuration for testing.
export SafeHarborPublicHostname=$1
export TestSuite=all
export RandomString=alkejfa4ak0s3
export DataVolMountPoint=/home/centos/safeharbordata  # this gets mapped to the container /safeharbor/data directory.
export registryUser=safeharbor
export registryPassword=gksspie8a
export postgresPassword=4word2day
export SafeHarborPort=6000
export RegistryPort=5000
export ScaledMarketsRegistryNamespace=500058573678.dkr.ecr.us-east-1.amazonaws.com
export SafeHarborImageName=$ScaledMarketsRegistryNamespace/safeharborserver

# Make sure docker is running.
#sudo service docker restart

# Deploy and test.
pushd $DeployDir
./createdeployenv.sh									# Create a test env.
./deploy.sh												# Deploy to the test env.
echo Running tests...
$BuildDir/test.sh $1 $SafeHarborPort $TestSuite			# Run tests.
popd
