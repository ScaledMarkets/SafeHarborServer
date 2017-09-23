#!/bin/sh

# This script deploys the SafeHarborServer in a standalone test mode that does not
# attempt to access redis, clair, postgres, or a registry.

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

# Get images:
sudo docker pull $SafeHarborImageName

# Start SafeHarborServer.
sudo docker run --name safeharbor \
	-d -p $SafeHarborPort:$SafeHarborPort \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-e RandomString="$RandomString" \
	-e SafeHarborPublicHostname="127.0.0.1" \
	$SafeHarborImageName /safeharbor/safeharbor \
	-debug -port=$SafeHarborPort -secretkey=jafakeu9s3ls -toggleemail -stubs -noregistry -inmem -host=$SafeHarborServerHost

sudo docker run --name safeharbor -d -e RandomString="$RandomString" -e SafeHarborPublicHostname="127.0.0.1" -p $SafeHarborPort:$SafeHarborPort -v $DataVolMountPoint:/safeharbor/data -v /var/run/docker.sock:/var/run/docker.sock $SafeHarborImageName /safeharbor/safeharbor -debug -port=$SafeHarborPort -secretkey=jafakeu9s3ls -toggleemail -stubs -noregistry -inmem -host=$SafeHarborServerHost
sudo docker run -it --name safeharbor -e RandomString="$RandomString" -e SafeHarborPublicHostname="127.0.0.1" -p $SafeHarborPort:$SafeHarborPort -v $DataVolMountPoint:/safeharbor/data -v /var/run/docker.sock:/var/run/docker.sock $SafeHarborImageName bash

./safeharbor -debug -port=6000 -secretkey=jafakeu9s3ls -toggleemail -stubs -noregistry -inmem -host="127.0.0.1"
./safeharbor -debug -port=6000 -secretkey=jafakeu9s3ls -toggleemail -stub clair -stub openscap -noregistry -inmem -host="127.0.0.1"

# For debugging:
# Start container but don't start SafeHarborServer.
# sudo docker run --net=host -it -p $SafeHarborPort:$SafeHarborPort -w=/safeharbor/ -v $DataVolMountPoint:/safeharbor/data -v /var/run/docker.sock:/var/run/docker.sock $SafeHarborImageName bash
