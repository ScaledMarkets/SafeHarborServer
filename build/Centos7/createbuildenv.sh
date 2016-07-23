# The machine must have docker and docker Compose installed on Centos7.
# For Docker, see https://docs.docker.com/engine/installation/linux/centos/
# For Compose, see https://github.com/docker/compose/releases

# ONE TIME: Dev env setup-------------------------------------------------------

# Install development tools on centos7:
sudo yum install go
sudo yum install git

# Install test suite.
pushd $PROJECTROOT/..
sudo git clone https://github.com/Scaled-Markets/TestSafeHarborServer.git
popd
