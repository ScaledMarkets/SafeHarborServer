#!/bin/sh

source env.sh

# Configure host.
sudo mkdir -p $DataVolMountPoint

# Create directories needed by docker Registry.
mkdir -p $DataVolMountPoint/registryauth
mkdir -p $DataVolMountPoint/registrydata
mkdir -p $DataVolMountPoint/registrycerts

# Create the safeharbor user.
....

# Give the safeharbor user privilege to access /var/run/docker.sock.
# (See http://www.atrixnet.com/allow-an-unprivileged-user-to-run-a-certain-command-with-sudo/ )
....

