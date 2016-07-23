# The machine must have docker and docker Compose installed on Centos7.
# For Docker, see https://docs.docker.com/engine/installation/linux/centos/
# For Compose, see https://github.com/docker/compose/releases

# ONE TIME: Dev env setup-------------------------------------------------------

# Install development tools on centos7:
sudo yum install go
sudo yum install git

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

# Install test suite.
pushd $BuildDir/../../..
git clone https://github.com/Scaled-Markets/TestSafeHarborServer.git
popd
