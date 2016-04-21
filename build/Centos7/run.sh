#!/bin/sh
# Run SafeHarborServer, using the conf.json file in the current directory.

sudo nohup ./bin/safeharbor -debug -stubs -secretkey=jafakeu9s3ls -host=52.11.24.124 < /dev/null &



# For testing:
sudo bin/safeharbor -debug -inmem -stubs -secretkey=jafakeu9s3ls -host=52.38.232.142
