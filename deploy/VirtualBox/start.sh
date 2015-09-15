#!/bin/sh

source $(dirname $0)/safeharbor.conf

# Name of the env variable that SafeHarborServer expects to be set to point to
# the SafeHarborServer conf.json file.
export SafeHarborConfEnvVarName=SAFEHARBOR_CONFIGURATION_PATH

# Run the auth server
#echo Staring Auth Server in container, on port $CesantaPort...
#vagrant ssh -c "docker run \
#	--name docker_auth \
#	--restart=always \
#	-p $CesantaPort:$CesantaPort \
#	-v /var/log/docker_auth:/logs \
#	-v $CesantaConfDir:/config:ro \
#	-v $CesantaSSLDir:/ssl \
#	--restart=always \
#	$CesantaDockerImage /config/auth_config.yml"

# Run the SafeHarborServer.
echo Staring SafeHarborServer in container, on port $SafeHarborPort...
vagrant ssh -c "docker run \
	--name SafeHarborServer \
	--restart=always \
	-p $SafeHarborPort:$SafeHarborPort \
	-v /home/vagrant/safeharbor:/safeharbor \
	-e $SafeHarborConfEnvVarName=$SafeHarborConfPath \
	-w /safeharbor \
	$SafeHarborDockerImage /safeharbor/SafeHarborServer"
