#!/bin/sh

# This script prepares a host for deployment of the various components of the
# SafeHarbor server.

. env.sh
RegistryAuthDir=$DataVolMountPoint/registryauth
DataDir=$DataVolMountPoint/registrydata
CertDir=$DataVolMountPoint/registrycerts

# Configure host.
[ ! -d $DataVolMountPoint ] && mkdir -p $DataVolMountPoint

# Create directories needed by docker Registry (if they don't already exist).
[ ! -d $RegistryAuthDir ] && mkdir -p $RegistryAuthDir
[ ! -d $DataDir ] && mkdir -p $DataDir
[ ! -d $CertDir ] && mkdir -p $CertDir

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
