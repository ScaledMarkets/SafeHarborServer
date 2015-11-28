#!/bin/sh
# Run SafeHarborServer, using the conf.json file in the current directory.

sudo nohup ./SafeHarborServer -debug -noauthorization -secretkey=jafakeu9s3ls > log.out 2> log.err < /dev/null &
