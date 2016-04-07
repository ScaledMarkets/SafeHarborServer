#!/bin/sh

source env.sh

# Configure host:
sudo mkdir -p $DataVolMountPoint

# Create Docker Registry password file.
mkdir -p $DataVolMountPoint/registryauth
mkdir -p $DataVolMountPoint/registrydata
mkdir -p $DataVolMountPoint/registrycerts
