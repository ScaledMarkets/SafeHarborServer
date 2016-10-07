#!/bin/sh
# Create the tools server and security group. Leaves the server running.
# Run this locally.
# After creating the server, run provision-tools-server.sh on the tools server
# to configure it.

# Create a VM.
....AMI ID
RHEL-7.2_HVM_GA-20151112-x86_64-1-Hourly2-GP2 (ami-775e4f16)


# Create security groups needed for tools.
....Clair: 6060-6061
aws ec2 create-security-group --group-name .... --description "...."
