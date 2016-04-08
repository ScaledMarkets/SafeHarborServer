#!/bin/sh

source env.sh

# Configure host.
sudo mkdir -p $DataVolMountPoint

# Create directories needed by docker Registry.
mkdir -p $DataVolMountPoint/registryauth
mkdir -p $DataVolMountPoint/registrydata
mkdir -p $DataVolMountPoint/registrycerts
