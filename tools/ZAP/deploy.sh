#!/bin/sh
# Deploy a OWASP "ZAP" security scanner pod.
# IMPORTANT: For testing against an AWS server, submit a pen test request:
#	https://aws.amazon.com/security/penetration-testing/
# The machine must have docker and docker Compose installed on Centos7/RHEL7.
# Parameters:
#	TargetEnvConfig - Path of a file to source that sets configuration variables.
#	ZAPPort

if [ -z $2 ] 
then
	echo "ZAP/deploy.sh: Usage: ./deploy.sh <TargetEnvConfig> <ZAPPort>"
	exit 2
fi

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export ScriptDir=`pwd`
echo "ZAP/deploy.sh: ScriptDir=$ScriptDir"
popd

# Set configuration for the target deployment env.
source $1

# Set variables needed by the compose command and yaml file.
export ZAPPort=$2
ComposeFileName=docker-compose.yml
ProjectName=ZAP

# docker run -u zap -p 8080:8080 -i owasp/zap2docker-stable zap.sh -daemon -host 0.0.0.0 -port 8080

# sudo docker run -v $(pwd):/zap/wrk/:rw -t owasp/zap2docker-stable zap-baseline.py -t https://ibm.com -r testreport.html

echo "ZAP/deploy.sh: Running ecs-cli compose up..."
$ComposeCommand --project-name $ProjectName --file $ScriptDir/$ComposeFileName \
	$ComposeServiceCommand up
