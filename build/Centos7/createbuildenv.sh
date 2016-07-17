# Install docker on centos7: See https://docs.docker.com/engine/installation/linux/centos/

# Load configuration.
source $( dirname "${BASH_SOURCE[0]}" )/env.sh

# ONE TIME: Dev env setup-------------------------------------------------------

# Install development tools on centos7:
sudo yum install go
sudo yum install git

# Install test suite.
pushd $PROJECTROOT/..
sudo git clone https://github.com/Scaled-Markets/TestSafeHarborServer.git
popd
