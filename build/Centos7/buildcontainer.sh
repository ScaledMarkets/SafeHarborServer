# Compile the source code and package it as a container image.
# Run this on the machine that hosts the containers under test.
# Arguments:
#	$1 - Optional. If the first argument equals "TEST", then the code will be
#		compiled for testing, with full instrumentation, including code coverage.
# 
# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

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

# Push safeharborserver image to AWS registry:
docker push $SafeHarborImageName

popd
