#!/bin/sh

source $(dirname $0)/safeharbor.conf

# Name of the env variable that SafeHarborServer expects to be set to point to
# the SafeHarborServer conf.json file.
export SafeHarborConfEnvVarName=SAFEHARBOR_CONFIGURATION_PATH

# Run the auth server
vagrant ssh -c docker run $CesantaDockerImage \
	--name CesantaAuthServer
	-p $CesantaPort:$CesantaPort \
	-v /var/log/docker_auth:/logs \
	-v $CesantaConfDir:/config:ro \
	-v $CesantaSSLDir:/ssl \
	--restart=always \
	/config/auth_config.yml
		
# Run the SafeHarborServer.
vagrant ssh -c docker run $SafeHarborDockerImage \
	--name SafeHarborServer
	-p $SafeHarborPort:$SafeHarborPort \
	-e $SafeHarborConfEnvVarName=$SafeHarborConfPath \
	SafeHarborServer
