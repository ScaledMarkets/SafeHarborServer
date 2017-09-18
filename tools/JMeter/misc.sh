
# Miscellaneous commands to be run by hand, during testing.
# http://stackoverflow.com/questions/16618915/setting-up-jmeter-for-distributed-testing-in-aws-with-connectivity-issues
# https://jmeter.apache.org/usermanual/jmeter_distributed_testing_step_by_step.pdf
# https://jmeter.apache.org/
# https://jmeter.apache.org/usermanual/remote-test.html
# http://www.tutorialspoint.com/jmeter/jmeter_webservice_test_plan.htm

export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
export AWS_DEFAULT_REGION=us-west-2
export AWS_REGION=us-west-2
export EC2_LOCAL_PEM_FILE_PATH=alethixkey.pem

export JMETER_HOME=~/apache-jmeter-3.0
export PerfPlanFilePath=acttestplan.jmx
export TestResultsDir=test_results

./run-perf-test.sh 1 1099 5050 5050 TestPlan.test.jmx perf_slave_instance_ids.csv true

cmd="SERVER_PORT=$RMIRegistryPort"
cmd="$cmd nohup $JMETER_HOME/bin/jmeter-server -n"
cmd="$cmd -Djava.rmi.server.hostname=$SlaveIpAddr"
cmd="$cmd -Dserver.rmi.localport=$JMeterSlavePort"
#cmd="$cmd -Dclient.rmi.localport="


===========================
export RMIRegistryPort=1099
export JMeterControllerCallbackPort=5050
export SlaveServerPort=5050
export JMeterStdout=jmeter.stdouterr.log

# For VirtualBox:
export ControllerIPAddr=192.168.56.104
export SlaveIpsCommaSep=192.168.56.103
export SlaveIP=192.168.56.103

# For AWS:
export ControllerIPAddr=172.31.34.0
export SlaveIpsCommaSep=172.31.17.9
export SlaveIP=172.31.17.9

echo $RMIRegistryPort
echo $JMeterControllerCallbackPort
echo $SlaveServerPort
echo $ControllerIPAddr
echo $SlaveIpsCommaSep
echo $SlaveIP

Slave:
SERVER_PORT=1099 \
apache-jmeter-3.0/bin/jmeter-server \
-Djava.rmi.server.hostname=$SlaveIP \
-Dclient.rmi.localport=5050 \
-Dserver.rmi.localport=5050

nohup /usr/local/apache-jmeter-3.0/bin/jmeter-server -n \
-Djava.rmi.server.hostname=$SlaveIp \
-Dserver.rmi.localport=5050 \
-Dclient.rmi.localport=5050 &> $JMeterStdout < /dev/null &

git clone https://github.com/alethix/tools.git

Controller:
/usr/local/apache-jmeter-3.0/bin/jmeter -n \
-t tools/JMeter/acttestplan.jmx \
-Djava.rmi.server.hostname=$ControllerIPAddr \
-Dclient.rmi.localport=$JMeterControllerCallbackPort \
-Dserver.rmi.localport=$SlaveServerPort \
-R$SlaveIpsCommaSep \
-lresults.jtl
	#-o test_results \
	#-Dsummariser.log=false

	
java -jar /usr/local/apache-jmeter-3.0/bin/ApacheJMeter.jar -n \
	-t perfplan.jmx \
	-Djava.rmi.server.hostname=172.31.34.0 \
	-Dclient.rmi.localport=5050 \
	-R172.31.17.9 \
	-lresults.csv
	
	
	
	
	