#!/bin/sh
# Create a set of JMeter 'slave' pods. The machine running this script is the master.
# This script can be run on the Jenkins server.
# The slaves generate load, and the master controls the slaves and collects results.
# This machine must have docker installed on Centos7/RHEL7.
# This machine must have AWS EC2 command line tools installed.
# This machine must have Java installed.
# Env variables that must be set:
#	PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH
#	EC2_LOCAL_PEM_FILE_PATH.
#	AWS_ACCESS_KEY_ID
#	AWS_SECRET_ACCESS_KEY
#	AWS_REGION
# Parameters:
#	TargetEnvConfig - Path of a file to source that sets configuration variables.
#	CreateSlaves - If true, then create slaves. Otherwise, use the slaves listed
#		in the file $PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH.
#	NoOfSlaves
#	LeaveSlavesRunning - If specified, the slaves are not stopped before exiting.
# Ref: https://jmeter.apache.org/usermanual/jmeter_distributed_testing_step_by_step.pdf
# Ref: https://jmeter.apache.org/
# Ref: https://jmeter.apache.org/usermanual/remote-test.html
# Ref: http://www.tutorialspoint.com/jmeter/jmeter_webservice_test_plan.htm

if [ -z $3 ] 
then
	echo "JMeter/deploy.sh: Usage: ./deploy.sh <TargetEnvConfig> <CreateSlaves> <NoOfSlaves> [<LeaveSlavesRunning>]"
	exit 3
fi

# Check environment.
if [ -z "$PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH" ]; then
	echo "JMeter/deploy.sh: PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH not set"
	exit 2
fi

if [ -z "$EC2_LOCAL_PEM_FILE_PATH" ]; then
	echo "JMeter/deploy.sh: EC2_LOCAL_PEM_FILE_PATH not set"
	exit 2
else
	echo "EC2_LOCAL_PEM_FILE_PATH=$EC2_LOCAL_PEM_FILE_PATH"
fi
echo "---------"

# Get script directory.
pushd $( dirname "${BASH_SOURCE[0]}" )
export ScriptDir=`pwd`
echo "JMeter/deploy.sh: ScriptDir=$ScriptDir"
popd

# Set configuration for the target deployment env.
source $1

# Set variables.
export CreateSlaves=$2
export NoOfSlaves=$3
export LeaveSlavesRunning=$4

# Set constants.
ComposeFileName=docker-compose.yml
ProjectName=JMeter
SshOptions="-i $EC2_LOCAL_PEM_FILE_PATH -o StrictHostKeyChecking=no "
echo "SshOptions=$SshOptions"

if [ $CreateSlaves == "true" ]; then
	# Create the slaves.
	# Slaves must have a security group that allows SSH (port 22) and RMI (port 1099).
	# Ref: http://docs.aws.amazon.com/cli/latest/reference/ec2/run-instances.html
	# Ref: https://coderwall.com/p/ndm54w/creating-an-ec2-instance-in-a-vpc-with-the-aws-command-line-interface
	echo "Creating slaves..."
	SLAVE_IMAGE_AMI=ami-775e4f16
	KeyName=alethixkey
	SecurityGroupId=sg-4e373c28
	SubnetId=subnet-dd6918b9
	InstanceIds=`aws \
		--query 'Instances[*].InstanceId' \
		--output text \
		ec2 run-instances \
		--image-id $SLAVE_IMAGE_AMI \
		--count $NoOfSlaves \
		--instance-type t2.medium \
		--key-name $KeyName \
		--security-group-ids $SecurityGroupId \
		--subnet-id $SubnetId`
	echo "InstanceIds=$InstanceIds"
	
	# Save the instance Ids in a file called $PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH, in this dir,
	# replacing the tab separators with commas.
	echo ${InstanceIds//$'\t'/","} > $PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH
	
	# Tag the slaves.
	echo "Tagging slaves..."
	N=0
	for id in $InstanceIds ; do
		let N=N+1
		aws ec2 create-tags --resources $id --tags Key=Name,Value=JMeterSlave$N
	done
fi

# Read the instance Ids.
echo "Obtaining slave Ids..."
if [ ! -f $PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH ]; then
	echo "JMeter/deploy.sh: File containing slave instance Ids not found. Did you specify CreateSlaves=true?"
	exit 2
fi
SlaveInstanceCommaSep=`cat $PERF_SLAVE_INSTANCE_ID_CSV_FILE_PATH`
Status=$?
if [ $Status -ne 0 ]; then
	echo "JMeter/deploy.sh: Error reading slave instance Id file"
	exit $Status
fi

# Construct an array of available slave instance Ids.
SlaveInstanceIds="${SlaveInstanceCommaSep//,/ }"
# Replaces all occurrences of ',' (the initial // means global replace) in the variable
# with ' ' (a single space), then interprets the space-delimited string as an array
# (that's what the surrounding parentheses do).
Status=$?
if [ $Status -ne 0 ]; then
	echo "JMeter/deploy.sh: Error parsing instance Ids: $SlaveInstanceCommaSep"
	exit $Status
fi

# Provision Java SE (7 or later) onto this node (needed by JMeter).
#....if java is not installed,
#sudo yum -y install java-1.7.0-openjdk
# JAVA_HOME=/usr/lib/jvm/jre-1.7.0-openjdk.x86_64

# Provision/configure jmeter on the master (this node).
echo "Downloading JMeter onto master..."
curl -L -o apache-jmeter-3.0.zip http://www.webhostingjams.com/mirror/apache/jmeter/binaries/apache-jmeter-3.0.zip
unzip -qq -o apache-jmeter-3.0.zip  # Installs at ~/apache-jmeter-3.0
rm apache-jmeter-3.0.zip
sudo mv apache-jmeter-3.0 /usr/local

OverallExitStatus=0

N=0
for id in $SlaveInstanceIds ; do
	let N=N+1
	
	# Get slave IP address.
	echo "aws ec2 describe-instances --instance-ids $id --query "Reservations[*].Instances[*].PrivateIpAddress" --output=text"
	ip=`aws ec2 describe-instances --instance-ids $id --query "Reservations[*].Instances[*].PrivateIpAddress" --output=text`
	Status=$?
	if [ $Status -ne 0 ]; then
		echo "JMeter/deploy.sh: Error obtaining IP address of instance $id"
		exit $Status
	fi

	# Provision Java SE (7 or later) onto each slave (needed by JMeter).
	echo "Installing Java on slave $N ($ip)..."
	ssh -t -t $SshOptions ec2-user@$ip "sudo yum -y install java-1.7.0-openjdk"
	# JAVA_HOME=/usr/lib/jvm/jre-1.7.0-openjdk.x86_64
	
	# Provision unzip onto slave.
	echo "Installing unzip on slave $N..."
	ssh -t -t $SshOptions ec2-user@$ip "sudo yum -y install unzip"

	# Provision JMeter onto each slave.
	echo "Installing JMeter on slave $N..."
	ssh -t -t $SshOptions ec2-user@$ip "curl -L -o apache-jmeter-3.0.zip http://www.webhostingjams.com/mirror/apache/jmeter/binaries/apache-jmeter-3.0.zip"
	ssh -t -t $SshOptions ec2-user@$ip "unzip -qq -o apache-jmeter-3.0.zip"
	sudo mv apache-jmeter-3.0 /usr/local
	
	# Provision git onto slave.
	echo "Installing git on slave $N..."
	ssh -t -t $SshOptions ec2-user@$ip "sudo yum -y install git"
	
	# Disable firewall on each slave. All must also be in the same subnet.
	echo "Disabling firewall on slave $N..."
	ssh -t -t $SshOptions ec2-user@$ip "sudo service iptables stop"
	
	# If LeaveSlavesRunning is not set, then put slaves to sleep.
	if [ -z $LeaveSlavesRunning ]; then
		# Ref: http://docs.aws.amazon.com/cli/latest/reference/ec2/stop-instances.html
		echo "stopping slave $N..."
		aws ec2 stop-instances --force --instance-ids $id
		ExitStatus=$?
		if [ $ExitStatus -ne 0 ]; then
			OverallExitStatus=$ExitStatus
			echo "Error putting slave $N to sleep"
		fi
	fi
done
echo "JMeter/deploy.sh: Finished creating slaves; exit status=$OverallExitStatus."

exit $OverallExitStatus
