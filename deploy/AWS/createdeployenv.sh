#!/bin/sh

source env.sh

# Configure host.
sudo mkdir -p $DataVolMountPoint

# Create directories needed by docker Registry.
mkdir -p $DataVolMountPoint/registryauth
mkdir -p $DataVolMountPoint/registrydata
mkdir -p $DataVolMountPoint/registrycerts

# Retrieve/update images that are needed.
sudo docker pull docker.io/redis
sudo docker pull docker.io/postgres
sudo docker pull quay.io/coreos/clair
sudo docker pull docker.io/registry:2
sudo docker pull $SafeHarborImageName

# Create the safeharbor user.
#sudo useradd -g safeharbor safeharbor

# Add the safeharbor user to the docker group so that it can access /var/run/docker.sock.
#usermod -a -G docker safeharbor
