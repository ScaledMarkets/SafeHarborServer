#!/bin/sh

# This script prepares a host for deployment of the various components of the
# SafeHarbor server. Before running this script, set the required environment
# variables, either by running env.sh for the target environment, followed by
# running build/common/env.sh.
# This script must be run on the server host before running the all/deploy.sh
# or the Compose/deploy.sh file.

if [ -z "$TARGET_ENV_CONFIGURED" ]
then
	echo "env.sh has not been run for the target env"
	exit 1
fi

# Create directories needed by docker Registry (if they don't already exist).
[ ! -d $DataVolMountPoint ] && mkdir -p $DataVolMountPoint
[ ! -d $RegistryAuthDir ] && mkdir -p $RegistryAuthDir
[ ! -d $DataDir ] && mkdir -p $DataDir
[ ! -d $CertDir ] && mkdir -p $CertDir
[ ! -d $ClairDir ] && mkdir -p $ClairDir
[ ! -d $PostgresDir ] && mkdir -p $PostgresDir
[ ! -d $RedisConfigDir ] && mkdir -p $RedisConfigDir
[ ! -d $RedisDataDir ] && mkdir -p $RedisDataDir

# Retrieve/update images that are needed.
sudo docker pull docker.io/redis
sudo docker pull docker.io/postgres
sudo docker pull quay.io/coreos/clair
sudo docker pull docker.io/registry:2
sudo docker pull $SafeHarborImageName

# Install Twistlock



# Create the safeharbor user.
#sudo useradd -g safeharbor safeharbor

# Add the safeharbor user to the docker group so that it can access /var/run/docker.sock.
#usermod -a -G docker safeharbor

# http://www.linuxjournal.com/magazine/work-shell-handling-errors-and-making-scripts-bulletproof
