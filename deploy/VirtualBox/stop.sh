#!/bin/sh

source $(dirname $0)/safeharbor.conf

vagrant ssh -c docker stop CesantaAuthServer
vagrant ssh -c "docker stop SafeHarborServer"
