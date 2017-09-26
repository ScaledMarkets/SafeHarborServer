#!/bin/sh

# DEPLOY SAFE HARBOR SERVER AND ITS COMPONENTS----------------------------------
# This script uses docker directly. One can instead use the deploy script in
# deploy/Compose.
#
# IMPORTANT: This script does not produce a secure deployment - it is for
# testing only. Use one of the orchestration scripts for production deployment.
#
# Parameter (optional): 'native' - If specified, then run SafeHarborServer as an
# ordinary application instead of as a container.

if [ -z "$TARGET_ENV_CONFIGURED" ]
then
	echo "env.sh has not been run for the target env"
	exit 1
fi

# Add credentials to environment.
# To do: This is very insecure. This is why this script should only be used for testing.
. $ScaledMarketsCredentialsDir/SetDockerhubCredentials.sh
. $SafeHarborCredentialDir/SetEmailServicePassword.sh
. $SafeHarborCredentialDir/SetPostgresPassword.sh
. $SafeHarborCredentialDir/SetRedisPassword.sh
. $SafeHarborCredentialDir/SetRegistryPassword.sh
. $SafeHarborCredentialDir/SetSafeHarborRandomString.sh
. $SafeHarborCredentialDir/SetTwistlockCredentials.sh

# Start redis (needed by SafeHarborServer).
cp $PROJECTROOT/deploy/all/redis.conf $RedisConfigDir
sudo docker run --name redis --net=host -d -v $RedisConfigDir:/config -v $RedisDataDir:/data redis --appendonly yes

# Start postgres (needed by Clair).
cp $PROJECTROOT/deploy/all/postgresql.conf $PostgresDir
#sudo docker run -d -p 5432:5432 --net=host -v $PostgresDir:/config -e POSTGRES_PASSWORD=$postgresPassword -e PGPASSWORD=$postgresPassword postgres
sudo docker run -d --name postgres -e POSTGRES_PASSWORD="" -p 5432:5432 postgres:9.6

# Start Clair (needed by SafeHarborServer).
# https://github.com/coreos/clair/tree/release-2.0#docker
cp $PROJECTROOT/deploy/all/clairconfig.yaml $ClairDir
#sudo docker run --name clair --net=host -p 6060:6060 -p 6061:6061 -v $ClairDir:/config:ro -e POSTGRES_PASSWORD=$postgresPassword quay.io/coreos/clair:latest -config=/config/clairconfig.yaml
sudo docker run -d --name clair -p 6060-6061:6060-6061 -v `pwd`:/config quay.io/coreos/clair-git:latest -config=/config/clairconfig.yaml

# Start Twistlock server and provide bearer token to activate license.
sudo $TwistlockDir/twistlock.sh -s onebox
curl -sSL -k --header "authorization: Bearer $TwistlockBearerToken" https://127.0.0.1:8083/api/v1/scripts/defender.sh | sudo bash -s -- -c "127.0.0.1" -d "none"


# Start OpenScap scanning slave.
#sudo docker run --name scap --net=host -d -v ....

# Create Docker Registry password file (needed by Docker Registry).
docker run --entrypoint htpasswd docker.io/registry:2 -Bbn $registryUser $registryPassword > $DataVolMountPoint/registryauth/htpasswd

# Start our own Docker Registry (needed by SafeHarborServer).
# For TLS, we will also need these:
#	-v $DataVolMountPoint/registrycerts:/certs \
#	-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt \
#	-e REGISTRY_HTTP_TLS_KEY=/certs/domain.key \
sudo docker run --name registry --net=host -d -p $RegistryPort:$RegistryPort \
	-v $DataVolMountPoint/registryauth:/auth \
	-v $DataVolMountPoint/registrydata:/var/lib/registry
	-e "REGISTRY_AUTH=htpasswd" \
	-e "REGISTRY_AUTH_HTPASSWD_REALM=Registry Realm" \
	-e "REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd" \
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
if [ $1 -eq 'native' ]
then # merely run the server as an app - don't run it as a container. This will block.
	if [ -z "$SAFEHARBOR_CONFIGURATION_PATH" ]
	then
		echo "SAFEHARBOR_CONFIGURATION_PATH needs to be set"
		exit
	else
		bin/safeharbor -debug -inmem -stubs -secretkey=$SafeHarborSecret -host=127.0.0.1
	fi
else # run as a container.
	#sudo docker run --net=host -d -p $SafeHarborPort:$SafeHarborPort -v $DataVolMountPoint:/safeharbor/data $SafeHarborImageName /safeharbor/safeharbor -debug -secretkey=jafakeu9s3ls -port=6000
	sudo docker run -i -t --name safeharborserver --net=host -p $SafeHarborPort:$SafeHarborPort \
		-v $DataVolMountPoint:/safeharbor/data \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e RandomString="$RandomString" \
		-e SafeHarborPublicHostname="127.0.0.1" \
		$SafeHarborImageName bash
fi
