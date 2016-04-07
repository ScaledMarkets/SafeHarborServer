#!/bin/sh

# This script deploys the SafeHarborServer in a standalone test mode that does not
# attempt to access redis, clair, or postgres.

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

source env.sh

# Get images:
sudo docker pull $SafeHarborImageName

# Start SafeHarborServer.
#sudo docker run --net=host -d -p 6000:6000 -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafa -port=6000 -inmem -stubs
sudo docker run --net=host -it -p 6000:6000 -w=/safeharbor/ -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName bash

/safeharbor/safeharbor -debug -secretkey=jafa -port=6000 -inmem -stubs -noregistry
