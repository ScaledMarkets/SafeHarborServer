#!/bin/sh
# Configure a build and test environment.
# Run this on a target architecture machine.
# The machine must have docker and docker Compose installed on Centos7.
# For Docker, see https://docs.docker.com/engine/installation/linux/centos/
# For Compose, see https://github.com/docker/compose/releases

# ONE TIME: Dev env setup-------------------------------------------------------

# Install development tools on centos7:
sudo yum install -y go
sudo yum install -y git
sudo yum install -y golang-cover.x86_64

# Set build location.
pushd $( dirname "${BASH_SOURCE[0]}" )
export BuildDir=`pwd`
echo BuildDir=$BuildDir
popd
pushd $BuildDir/../../deploy/Compose
export DeployDir=`pwd`
popd
echo DeployDir=$DeployDir

# Fix file ownership.
sudo chown centos:centos $BuildDir/*
sudo chown centos:centos $DeployDir/*
sudo chmod +x $BuildDir/*.sh
sudo chmod +x $DeployDir/*.sh

# Create docker group.
sudo groupadd docker
sudo gpasswd -a centos docker

# Install test suite.
pushd $BuildDir/../../..
git clone https://github.com/Scaled-Markets/TestSafeHarborServer.git
popd
