source env.sh

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# BUILD CONTAINER IMAGE---------------------------------------------------------

# Build the safeharborserver executable.
cd ~/SafeHarborServer
sudo git pull
sudo make compile

# Build safeharborserver image:
sudo mv bin/safeharbor build/Centos7
cd build/Centos7
sudo docker build --tag=$SafeHarborImageName .

# Push safeharborserver image to AWS registry:
sudo docker push $SafeHarborImageName
