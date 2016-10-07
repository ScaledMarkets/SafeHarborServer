#!/bin/sh
# Perform a Clair scan of an image.
# Assumes that the docker client and daemon are installed on this machine.
# Assumes that the analyze-local-images tool is installed on this machine,
# and that Clair is running.
# Parameters:
#	ServerAndAccount - e.g., docker.io/alethix
#	UserId - User Id for the docker registry that contains the image.
#	Password - Password for the above user Id.
#	ImageName - Name of the image to be scanned.
# Example:
#	./clair_scan.sh docker.io/alethix <dockerhub-userid> <pswd> php:latest
# When a scan finds no vulnerabilities, analyze-local-images.sh will return,
#	Success! No vulnerabilities were detected in your image

if [ -z $4 ] 
then
	echo "Clair/clair_scan.sh: Usage: ./clair_scan.sh <server-domain-name>/<acct-name> <registry-userid> <pswd> <ImageName>"
	exit 2
fi

ServerAndAccount=$1
UserId=$2
Password=$3
ImageName=$4

echo "Logging into $ServerAndAccount"
docker login -u $UserId -p $Password $ServerAndAccount
Status=$?
if [ $Status -ne 0 ]; then
	echo "Clair/clair_scan.sh: docker registry login failed"
	exit $Status
fi

# Note: If the following pull fails, check that the above login added the
# appropriate credentials to $HOME/.docker/config.json
echo "Pulling image $ImageName"
docker pull $ImageName
Status=$?
if [ $Status -ne 0 ]; then
	echo "Clair/clair_scan.sh: docker pull failed"
	exit $Status
fi

echo "Calling analyze-local-images $ImageName..."
echo "home=$HOME"
GOPATH=$HOME /home/ec2-user/bin/analyze-local-images $ImageName
#GOPATH=$HOME analyze-local-images $ImageName
Status=$?
if [ $Status -ne 0 ]; then
	echo "Clair/clair_scan.sh: analyze-local-images failed"
	exit $Status
fi
