#!/bin/sh
# Compile the source code and package it as a container image.
# Run this on a target architecture machine.
# Arguments:
#	$1 - Optional. If the first argument equals "TEST", then the code will be
#		compiled for testing, with full instrumentation, including code coverage.

pushd $PROJECTROOT

# Build the safeharborserver executable.
git pull

if [ $1 -eq 'TEST' ]
then
	make cover  # compile with test coverage instrumentation
else
	make compile
fi

# Build safeharborserver image:
cp bin/safeharbor build/Centos7
cd build/Centos7

if [ $1 -eq 'TEST' ]
then
	docker build --file Dockerfile.test --tag=$SafeHarborImageName .
else
	docker build --tag=$SafeHarborImageName .
fi

# Push safeharborserver image to registry:
source $ScaledMarketsCredentialsDir/SetDockerhubCredentials.sh
docker login -u $DockerhubUserId -p $DockerhubPassword
docker push $SafeHarborImageName
docker logout

popd
