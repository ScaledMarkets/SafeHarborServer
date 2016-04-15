#!/bin/sh
# Run SafeHarborServer, using the conf.json file in the current directory.

sudo nohup ./bin/safeharbor -debug -stubs -secretkey=jafakeu9s3ls -host=52.11.24.124 > log.out 2> log.err < /dev/null &
