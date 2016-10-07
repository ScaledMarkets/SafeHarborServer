# Deployment configuration for ECS test env.

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export EnvDir=`pwd`
echo "Environments/LocalTest_JMeterDeployConfig.sh: EnvDir=$EnvDir"
popd

# Set variables for the environment.
source $EnvDir/LocalTest_Config.sh

# Set JMeter-specific variables.
export PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH=$EnvDir/../JMeter/perf_slave_instance_ids.csv
