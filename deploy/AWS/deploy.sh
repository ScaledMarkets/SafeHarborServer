#!/bin/sh

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------

# MANUAL STEP: Log into registry instance in AWS:
# 1. Get the AWS container registry login command by executing "aws ecr get-login" locally.
# 2. Then paste that command into the shell that will run this script.
# We can eventually automate the above by installing the AWS command tools on
# the deployment machine and then running "sudo `aws ecr get-login`

source $( dirname "${BASH_SOURCE[0]}" )/env.sh

# Start redis (needed by SafeHarborServer).
sudo docker run --name redis --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config -v /home/centos/safeharbordata:/data docker.io/redis redis-server --appendonly yes /config/redis.conf

# Start postgres (needed by Clair).
sudo docker run --name postgres --net=host -d -e POSTGRES_PASSWORD=4word2day -d docker.io/postgres

# Start Clair (needed by SafeHarborServer).
sudo docker run --name clair --net=host -d -v /home/centos/SafeHarborServer/build/Centos7:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml

# Create Docker Registry password file (needed by Docker Registry).
docker run --entrypoint htpasswd docker.io/registry:2 -Bbn $registryUser $registryPassword > $DataVolMountPoint/registryauth/htpasswd

# Start our own Docker Registry (needed by SafeHarborServer).
# For TLS, we will also need these:
#	-v $DataVolMountPoint/registrycerts:/certs \
#	-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
#	-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
sudo docker run --name registry --net=host -d -p $RegistryPort:$RegistryPort \
	-v $RegistryPath/registryauth:/auth \
	-v $DataVolMountPoint/registrydata:/var/lib/registry
	-e "REGISTRY_AUTH=htpasswd" \
	-e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
	-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
	docker.io/registry:2

# Start SafeHarborServer.
# Note that we must mount the /var/run/docker.sock unix socket in the container
# so that the container can access the docker engine. If we do that, then we do not
# need to give the container SYS_RAWIO privilege. Regarding privileges, see:
#	https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities
#	http://www.atrixnet.com/allow-an-unprivileged-user-to-run-a-certain-command-with-sudo/
#	http://stackoverflow.com/questions/1956732/is-it-possible-to-configure-linux-capabilities-per-user
#	http://www.friedhoff.org/posixfilecaps.html
#	https://blog.yadutaf.fr/2016/04/14/docker-for-your-users-introducing-user-namespace/
#	http://www.projectatomic.io/blog/2015/08/why-we-dont-let-non-root-users-run-docker-in-centos-fedora-or-rhel/
#sudo docker run --net=host -d -p $SafeHarborPort:$SafeHarborPort -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafakeu9s3ls -port=6000
sudo docker run -i -t --name safeharborserver --net=host -p $SafeHarborPort:$SafeHarborPort \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	$SafeHarborImageName bash
