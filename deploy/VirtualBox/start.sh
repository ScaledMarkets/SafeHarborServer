#!/bin/sh

source $(dirname $0)/safeharbor.conf

# Name of the env variable that SafeHarborServer expects to be set to point to
# the SafeHarborServer conf.json file.
export SafeHarborConfEnvVarName=SAFEHARBOR_CONFIGURATION_PATH

# Run the auth server
vagrant ssh -c "docker run \
	--name docker_auth \
	--rm \
	--restart=always \
	-p $CesantaPort:$CesantaPort \
	-v /var/log/docker_auth:/logs \
	-v $CesantaConfDir:/config:ro \
	-v $CesantaSSLDir:/ssl \
	--restart=always \
	$CesantaDockerImage /config/auth_config.yml" &

#echo Auth server started in container

# Run the SafeHarborServer.
vagrant ssh -c "docker run \
	--name SafeHarborServer \
	--rm \
	--restart=always \
	-p $SafeHarborPort:$SafeHarborPort \
	-v /home/vagrant/safeharbor:/safeharbor \
	-e $SafeHarborConfEnvVarName=$SafeHarborConfPath \
	-w /safeharbor \
	$SafeHarborDockerImage /safeharbor/SafeHarborServer" &

echo SafeHarborServer started in container
