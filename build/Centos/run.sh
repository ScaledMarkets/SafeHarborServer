#!/bin/sh
# Run SafeHarborServer, using the conf.json file in the current directory.

sudo nohup ./SafeHarborServer --debug > log.out 2> log.err < /dev/null &
