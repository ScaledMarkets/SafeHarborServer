#!/bin/sh

source $(dirname $0)/safeharbor.conf

#vagrant ssh -c "docker stop docker_auth"
vagrant ssh -c "docker stop SafeHarborServer"
