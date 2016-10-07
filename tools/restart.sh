#!/bin/sh
# Re-start the Tools server.
# Run this locally to start the Tools server in AWS.
# Assumes that the tools server has been provisioned, via provision-tools-server.sh.
# Requires:
#	The AWS EC2 command line tools have been installed on this machine.

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export ScriptDir=`pwd`
echo "start.sh: ScriptDir=$ScriptDir"
popd

# Set configuration for the tools env.
source $ScriptDir/Environments/ECSTest_Config.sh

# Send request to AWS to start the tools server.
# This assumes that the AWS access key is in the $HOME/.aws directory for the user.
# Ref: http://docs.aws.amazon.com/AWSEC2/latest/CommandLineReference/ApiReference-cmd-StartInstances.html
ec2-start-instances --region $AWS_REGION \
	--access-key $AWS_ACCESS_KEY_ID --secret-key $AWS_SECRET_ACCESS_KEY \
	$EC2_TOOL_INSTANCE_ID

# Start docker on the tools server.
ssh -i $EC2_LOCAL_PEM_FILE_PATH ec2-user@$EC2_TEST_TOOL_ELASTIC_IP sudo service docker start
SshStatus=$?
if [ $SshStatus -ne 0 ]; then
	exit $SshStatus
fi

# Remove old containers.
ssh -i $EC2_LOCAL_PEM_FILE_PATH ec2-user@$EC2_TEST_TOOL_ELASTIC_IP sudo docker rm *
SshStatus=$?
if [ $SshStatus -ne 0 ]; then
	exit $SshStatus
fi

# We can now run the various tool deploy.sh scripts.

exit $SshStatus
