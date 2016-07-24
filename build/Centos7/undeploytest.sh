#!/bin/sh
# Create a test environment, run tests, and optionally tear down env if tests all pass.
# Arguments:
#	$1 - IP address of the test server.

pushd $( dirname "${BASH_SOURCE[0]}" )
export BuildDir=`pwd`
echo BuildDir=$BuildDir
popd
pushd $BuildDir/../../deploy/Compose
export DeployDir=`pwd`
popd
echo DeployDir=$DeployDir

pushd $DeployDir
./undeploy.sh								# Destroy test env.
popd
