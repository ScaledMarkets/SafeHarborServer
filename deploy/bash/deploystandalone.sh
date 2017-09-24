#!/bin/sh

# This script deploys the SafeHarborServer in a standalone test mode that does not
# attempt to access redis, clair, postgres, or a registry.

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

# Get images:
sudo docker pull $SafeHarborImageName

# Start SafeHarborServer.
sudo docker run -rm -d \
	--name safeharbor \
	-p $SafeHarborPort:$SafeHarborPort \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-e RandomString="$RandomString" \
	-e SafeHarborPublicHostname="127.0.0.1" \
	$SafeHarborImageName /safeharbor/safeharbor \
	-debug -port=$SafeHarborPort -secretkey=jafakeu9s3ls -toggleemail -stubs -noregistry -inmem -host=$SafeHarborServerHost

# For debugging:
# Start container but don't start SafeHarborServer.
# To run SafeHarborServer manually:
sudo docker run --rm -it \
	--name safeharbor \
	--net host \
	-p $SafeHarborPort:$SafeHarborPort \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-e RandomString="$RandomString" \
	-e SafeHarborPublicHostname="127.0.0.1" \
	$SafeHarborImageName bash

./safeharbor -debug -port=6000 -secretkey=jafakeu9s3ls -toggleemail -stub clair -stub openscap -noregistry -inmem -host="127.0.0.1"
