#!/bin/sh
# http://www.linuxjournal.com/magazine/work-shell-handling-errors-and-making-scripts-bulletproof

# Run this on the machine that will host the SafeHarborServer containers.
# The machine must have docker and docker Compose installed. For Compose,
# see https://github.com/docker/compose/releases

# Configure host.
mkdir -p $DataVolMountPoint

# Create directories needed by docker Registry.
mkdir -p $DataVolMountPoint/registryauth
mkdir -p $DataVolMountPoint/registrydata
mkdir -p $DataVolMountPoint/registrycerts

# Retrieve/update images that are needed.
docker pull docker.io/redis
docker pull docker.io/postgres
docker pull quay.io/coreos/clair
docker pull docker.io/registry:2
docker pull $SafeHarborImageName

# Create the safeharbor user.
#sudo useradd -g safeharbor safeharbor

# Add the safeharbor user to the docker group so that it can access /var/run/docker.sock.
#usermod -a -G docker safeharbor
