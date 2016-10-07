[JMeter](https://jmeter.apache.org/) is a performance testing tool.
### How JMeter Is Used - Concepts
JMeter can be used in many ways. We will be using it to generate REST from a set of virtual machines, all directed at a running instance of our application. JMeter calls the load generation machines *slaves*, and the controlling machine the *controller*. In our setup, the Jankins VM will be our controller.
### Installing JMeter
(*Pre-requisite:* Our performance testing scripts requires that the AWS EC2 command line tools are installed on the Jenkins server, so that EC2 instances can be started and stopped.)

JMeter can be installed by running the script `tools/JMeter/deploy.sh`. The script parameters are documented in the script. This script creates a number of slave machines, specified by a parameter. Those machines exist in a stopped state until they are started by a performance test run. They are stopped again at the end of the run. They are not containers - they are full VMs. Their AWS instance IDs are written to a file called `perf_slave_instance_ids.csv`. This file is read when a performance test is run.
### How To Load Test Our Application - Procedure
A performance test can be run by running one of the performance testing scripts in our projects. For example, a performance test can be run for the users-microservice by running
```
users-microservice/run-perf-test.sh
```
The script parameters are documented in the script.
