# For running server manually, for testing - not intended for production use.

source $( dirname "${BASH_SOURCE[0]}" )/env.sh

#nohup redis-server redis.conf > redis.out 2> redis.err < /dev/null &
#rm redis/redis.log
#rm redis/dump.rdb
#sudo -u postgres pg_ctl stop -D /usr/local/pgsql/data
#sudo nohup bin/safeharbor -debug -secretkey=jafakeu9s3ls -stubs > log.out 2> log.err < /dev/null &
#sudo docker run --name=postgres -e POSTGRES_PASSWORD=4word2day -d postgres
#sudo docker run --link postgres -p 6060:6060 -p 6061:6061 -v /home/centos:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml
#sudo nohup bin/safeharbor -debug -stubs -inmem -secretkey=jafakeu9s3ls -noregistry -host=52.38.155.244 < /dev/null &


# For Steve's test server:
sudo nohup bin/safeharbor -debug -stubs -secretkey=jafakeu9s3ls -noregistry -host=52.11.24.124 > log.out 2> log.err < /dev/null &

# For my testing:
sudo bin/safeharbor -debug -stubs -secretkey=jafakeu9s3ls -noregistry -inmem -host=52.40.36.44

# Containerized:
sudo docker run -i -t --net=host -p $SafeHarborPort:$SafeHarborPort \
	-v $DataVolMountPoint:/safeharbor/data \
	-v /var/run/docker.sock:/var/run/docker.sock \
	$SafeHarborImageName bash
