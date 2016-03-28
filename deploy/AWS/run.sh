#!/bin/sh

# To get redis:
docker pull dockerhub/redis

# Start redis.
sudo docker run --net=host -d -v /home/centos/SafeHarborServer:/data redis redis-server \
	--appendonly yes /data/redis.conf

# Start postgres.
sudo docker run --net=host -d -e POSTGRES_PASSWORD=4word2day -d postgres

# Start Clair.
sudo docker run --net=host -d -v /home/centos:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml

# Run SafeHarborServer, using the conf.json file in the current directory.
sudo docker run --net=host -d -p 6000:6000 -v /home/centos/SafeHarborServer:/ safeharbor \
	-debug -secretkey=jafakeu9s3ls -port=6000




#nohup redis-server redis.conf > redis.out 2> redis.err < /dev/null &
#rm redis/redis.log
#rm redis/dump.rdb
#sudo -u postgres pg_ctl stop -D /usr/local/pgsql/data
#sudo nohup bin/safeharbor -debug -secretkey=jafakeu9s3ls -stubs > log.out 2> log.err < /dev/null &
#sudo docker run --name=postgres -e POSTGRES_PASSWORD=4word2day -d postgres
#sudo docker run --link postgres -p 6060:6060 -p 6061:6061 -v /home/centos:/config:ro quay.io/coreos/clair:latest --config=/config/clairconfig.yaml
