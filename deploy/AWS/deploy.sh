#!/bin/sh

export ScaledMarketsRegistry=500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets
export SafeHarborImageName=$ScaledMarketsRegistry/safeharborserver


# Install docker on centos7:
# See https://docs.docker.com/engine/installation/linux/centos/

# Install development tools on centos7:
sudo yum install go
sudo yum install git

# Build the safeharborserver executable.
sudo git clone https://github.com/Scaled-Markets/SafeHarborServer.git
cd ~/SafeHarborServer
sudo git pull
sudo make compile

# Log into AWS container registry:
# Get the login command by executing "aws ecr get-login" locally.

# Build safeharborserver image:
sudo mv bin/safeharbor build/Centos7
cd build/Centos7
sudo docker build --tag=$SafeHarborImageName .

# Push safeharborserver image to AWS registry:
sudo docker push $SafeHarborImageName

# Get images:
sudo docker pull docker.io/redis
sudo docker pull docker.io/postgres
sudo docker pull quay.io/coreos/clair
#sudo docker pull 500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets/safeharborserver

# Configure host:
sudo mkdir /home/centos/safeharbordata

# Start redis.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config -v /home/centos/safeharbordata:/data docker.io/redis redis-server --appendonly yes /config/redis.conf

# Start postgres.
sudo docker run --net=host -d -e POSTGRES_PASSWORD=4word2day -d docker.io/postgres

# Start Clair.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml

# Start SafeHarborServer.
#sudo docker run --net=host -d -p 6000:6000 -v /home/centos/safeharbordata:/safeharbor $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafakeu9s3ls -port=6000
sudo docker run -i -t --net=host -p 6000:6000 -v /home/centos/safeharbordata:/safeharbor/data $SafeHarborImageName bash
