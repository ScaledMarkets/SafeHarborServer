# Deployment configuration for ECS test env.

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export EnvDir=`pwd`
echo "Environments/ECSTest_ZAPDeployConfig.sh: EnvDir=$EnvDir"
popd

# Set variables for the environment.
source $EnvDir/ECSTest_Config.sh
