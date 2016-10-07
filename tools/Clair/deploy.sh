#!/bin/sh
# Deploy a CoreOS "clair" container security scanner pod (clair and postgres).
# Run this on the Tools server.
# Ref: https://github.com/coreos/clair
# Parameters:
#	TargetEnvConfig - Path of a file to source that sets configuration variables.
#	PostgresPassword
# The machine must have docker and docker Compose installed on Centos7/RHEL7.
# For Docker, see https://docs.docker.com/engine/installation/linux/centos/
# For Compose, see https://github.com/docker/compose/releases
# For Compose with ECS, see http://docs.aws.amazon.com/AmazonECS/latest/developerguide/cmd-ecs-cli-compose.html
# For Postgres, see https://hub.docker.com/_/postgres/
# To deploy by hand,
#	sudo docker run --name postgres --network bridge -p 5432:5432 -e POSTGRES_PASSWORD=PostgresPassword -d docker.io/postgres
#	sudo docker run --name clair --network bridge --link postgres -p 6060-6061:6060-6061 -v /tmp:/tmp -v /home/ec2-user/tools/Clair:/config:ro -it quay.io/coreos/clair:v1.2.2 -config=/config/clairconfig.yaml
# To use Clair, see https://github.com/coreos/clair/tree/master/contrib/analyze-local-images

if [ -z $2 ] 
then
	echo "Clair/deploy.sh: Usage: ./deploy.sh <TargetEnvConfig> <PostgresPassword>"
	exit 2
fi

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export ScriptDir=`pwd`
echo "Clair/deploy.sh: ScriptDir=$ScriptDir"
popd

# Set configuration for the target deployment env.
source $1

# Install the command line tool for accessing Clair.
# We can then do (assuming that the user belongs to the docker group),
#	docker pull <image-name>
#	GOPATH=$HOME analyze-local-images <image-name>
# If you get "could not find layer", it is probably this issue:
#	https://github.com/coreos/clair/issues/190
# Check "docker logs clair" to see.
export GOROOT=/usr/local/go
export PATH=$PATH:$GOROOT/bin
GOPATH=$HOME /usr/local/go/bin/go get -u github.com/coreos/clair/contrib/analyze-local-images

# Set variables needed by the compose command and yaml file.
export PostgresPassword=$2
ComposeFileName=docker-compose.yml
ProjectName=Clair

#echo "Clair/deploy.sh: Running ecs-cli compose up..."
#$ComposeCommand --project-name $ProjectName --file $ScriptDir/$ComposeFileName \
#	$ComposeServiceCommand up

sudo docker run --name postgres --network bridge -p 5432:5432 -e POSTGRES_PASSWORD=PostgresPassword -d docker.io/postgres
ExitStatus=$?
if [ $ExitStatus -ne 0 ]; then
	echo "Failed starting postgres"
	exit $ExitStatus
fi
sudo docker run -d --name clair --network bridge --link postgres -p 6060-6061:6060-6061 -v /tmp:/tmp -v /home/ec2-user/tools/Clair:/config:ro -it quay.io/coreos/clair:v1.2.2 -config=/config/clairconfig.yaml
ExitStatus=$?
if [ $ExitStatus -ne 0 ]; then
	echo "Failed starting clair"
	exit $ExitStatus
fi
echo "Clair/deploy.sh: Started Clair, status $ComposeExitStatus."

exit $ExitStatus
