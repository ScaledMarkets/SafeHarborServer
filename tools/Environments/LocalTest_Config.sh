# Variables for local deployment.

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export ScriptDir=`pwd`
echo "Environments/LocalTest_Config.sh: ScriptDir=$ScriptDir"
popd

# Set variables specific to local deployment.
export ComposeCommand="docker-compose"
export ComposeServiceCommand=""
