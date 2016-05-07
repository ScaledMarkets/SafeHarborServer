source $( dirname "${BASH_SOURCE[0]}" )/env.sh

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# BUILD CONTAINER IMAGE---------------------------------------------------------

pushd $PROJECTROOT

# Build the safeharborserver executable.
sudo git pull
sudo make compile

# Build safeharborserver image:
sudo cp bin/safeharbor build/Centos7
cd build/Centos7
sudo docker build --tag=$SafeHarborImageName .

# Push safeharborserver image to AWS registry:
sudo docker push $SafeHarborImageName

popd
