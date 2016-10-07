# Variables unique to the region and cluster that is used for testing.
export AWS_REGION=us-west-2
export ECS_CLUSTER_NAME=dev-backend-services
export EC2_TOOL_INSTANCE_ID=i-014f311b7514c2a4a
export EC2_TEST_TOOL_ELASTIC_IP=52.40.82.135
export EC2_TEST_TOOL_PRIVATE_IP=172.31.20.21
export EC2_TEST_BUILD_ELASTIC_IP=52.40.101.126
export EC2_TEST_BUILD_PRIVATE_IP=172.31.34.0

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export EnvDir=`pwd`
echo "Environments/ECSTest_Config.sh: EnvDir=$EnvDir"
popd

# Variables for all of AWS.
source $EnvDir/ECS_Config.sh

if [ -z $AWS_ACCESS_KEY_ID ]
then
	echo "Environments/ECSTest_Config.sh: AWS_ACCESS_KEY_ID is not set"
	exit 2
fi

if [ -z $AWS_SECRET_ACCESS_KEY ]
then
	echo "Environments/ECSTest_Config.sh: AWS_SECRET_ACCESS_KEY is not set"
	exit 2
fi

# Configure the cluster settings.
# Requires AWS command line tools.
# Ref: http://docs.aws.amazon.com/cli/latest/userguide/installing.html
echo "PATH=$PATH"
ecs-cli configure --region $AWS_REGION --cluster $ECS_CLUSTER_NAME \
	--access-key $AWS_ACCESS_KEY_ID --secret-key $AWS_SECRET_ACCESS_KEY
Status=$?
if [ $Status -ne 0 ]; then
	echo "Failed in ecs-cli configure"
	exit $Status
fi
