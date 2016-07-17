#!/bin/sh

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

# MANUAL STEP: Log into registry instance in AWS:
# 1. Get the AWS container registry login command by executing "aws ecr get-login" locally.
# 2. Then paste that command into the shell that will run this script.
# We can eventually automate the above by installing the AWS command tools on
# the deployment machine and then running "sudo `aws ecr get-login`

# Load configuration.
source $( dirname "${BASH_SOURCE[0]}" )/env.sh

# Create Docker Registry password file (needed by Docker Registry).
docker run --entrypoint htpasswd docker.io/registry:2 -Bbn $registryUser $registryPassword > $DataVolMountPoint/registryauth/htpasswd

docker-compose up -d
