#!/bin/sh

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

source env.sh

# Get images:
sudo docker pull docker.io/redis
sudo docker pull docker.io/postgres
sudo docker pull quay.io/coreos/clair
sudo docker pull $SafeHarborImageName

# Configure host:
sudo mkdir -p /home/centos/safeharbordata

# Start redis.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config -v /home/centos/safeharbordata:/data docker.io/redis redis-server --appendonly yes /config/redis.conf

# Start postgres.
sudo docker run --net=host -d -e POSTGRES_PASSWORD=4word2day -d docker.io/postgres

# Start Clair.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml

# Start SafeHarborServer.
#sudo docker run --net=host -d -p 6000:6000 -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafakeu9s3ls -port=6000
sudo docker run -i -t --net=host -p 6000:6000 -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName bash
