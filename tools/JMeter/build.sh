#!/bin/sh
# NOT USED AT PRESENT. WILL USE THIS WHEN JMETER HAS BEEN CONTAINERIZED.

# Build JMeter container.
# Parameters:
#	JMETER_HOME - 
#	LOCALIP - 
#	JMeterImageName - 

if [ -z $3 ]
then
	echo "JMeter/build.sh: Usage: ./build.sh <JMETER_HOME> <LOCALIP> <JMeterImageName>"
	exit 2
fi

export JMETER_HOME=$1
export LOCALIP=$2
export JMeterImageName=$3

# Build the image.
echo "JMeter/build.sh: Running docker build..."
docker build --tag=$JMeterImageName .
BuildExitStatus=$?
echo "JMeter/build.sh: Finished running docker build: status $BuildExitStatus."
if [ $BuildExitStatus -ne 0 ]; then
	exit $BuildExitStatus
fi

# Push image to container image registry:
....log into remote registry.
docker push $JMeterImageName
PushExitStatus=$?
echo "JMeter/build.sh: Finished pushing image: status $PushExitStatus."

exit $PushExitStatus
