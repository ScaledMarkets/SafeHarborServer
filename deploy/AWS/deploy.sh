#!/bin/sh

# MANUAL STEP:
# Log into AWS container registry:
# Get the AWS container registry login command by executing "aws ecr get-login" locally.
# Then paste that command into the development env shell.

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

source env.sh

# Get images:
sudo docker pull docker.io/redis
sudo docker pull docker.io/postgres
sudo docker pull quay.io/coreos/clair
sudo docker pull $SafeHarborImageName

# Start redis.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config -v /home/centos/safeharbordata:/data docker.io/redis redis-server --appendonly yes /config/redis.conf

# Start postgres.
sudo docker run --net=host -d -e POSTGRES_PASSWORD=4word2day -d docker.io/postgres

# Start Clair.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml

# Create Docker Registry password file.
docker run --entrypoint htpasswd docker.io/registry:2 -Bbn $registryUser $registryPassword > $DataVolMountPoint/registryauth/htpasswd

# Start Docker Registry.
# Note that we must mount the /var/run/docker.sock unix socket in the container
# so that the container can access the docker engine.
sudo docker run --net=host -d -p 5000:5000 --name registry \
	-v $RegistryPath/registryauth:/auth \
	-v $DataVolMountPoint/registrydata:/var/lib/registry
	-v /var/run/docker.sock:/var/run/docker.sock \
	-u safeharbor \
	-e "REGISTRY_AUTH=htpasswd" \
	-e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
	-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
	#-v $DataVolMountPoint/registrycerts:/certs \
	#-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
	#-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
	docker.io/registry:2

# Start SafeHarborServer.
#sudo docker run --net=host -d -p 6000:6000 -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafakeu9s3ls -port=6000
sudo docker run -i -t --net=host -p 6000:6000 \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	$SafeHarborImageName bash
