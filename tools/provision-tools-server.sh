#!/bin/sh
# Set up the tools server. (To create the server, use create-tools-server.sh)
# Run this on the Tools server.
# Ref: http://docs.aws.amazon.com/AWSEC2/latest/CommandLineReference/ec2-clt.pdf

sudo yum -y update

# Install docker-engine.
sudo tee /etc/yum.repos.d/docker.repo <<-EOF
[dockerrepo]
name=Docker Repository
baseurl=https://yum.dockerproject.org/repo/main/centos/7
enabled=1
gpgcheck=1
gpgkey=https://yum.dockerproject.org/gpg
EOF
sudo yum -y install docker-engine
sudo service docker start

# Install docker compose.
curl -L https://github.com/docker/compose/releases/download/1.8.0/docker-compose-`uname -s`-`uname -m` > docker-compose
chmod +x docker-compose
sudo mv docker-compose /usr/local/bin/docker-compose

# Install git, unzip, and wget.
sudo yum install -y git
sudo yum install -y wget
sudo yum install -y unzip

# Install EPEL.
#sudo wget http://dl.fedoraproject.org/pub/epel/7/x86_64/e/epel-release-7-7.noarch.rpm
#sudo rpm -iUvh epel-release-7-7.noarch.rpm
#sudo rm epel-release-7-7.noarch.rpm -f

# Install golang.
sudo wget https://storage.googleapis.com/golang/go1.6.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.6.3.linux-amd64.tar.gz

# Add EC2 user to docker group.
sudo usermod -a -G docker ec2-user

# Install AWS command line tools.
curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
sudo ./awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws

# Install AWS EC2 command line tools.
curl -O http://s3.amazonaws.com/ec2-downloads/ec2-api-tools.zip
sudo mkdir /usr/local/ec2
sudo unzip ec2-api-tools.zip -d /usr/local/ec2

# Install ECS CLI.
# Ref: http://docs.aws.amazon.com/AmazonECS/latest/developerguide/ECS_CLI_installation.html
sudo curl -o /usr/local/bin/ecs-cli https://s3.amazonaws.com/amazon-ecs-cli/ecs-cli-linux-amd64-latest
sudo chmod +x /usr/local/bin/ecs-cli

# Install Java if it is not present (needed by EC2 command line tools).
if [ ! (which java) == "/usr/bin/java" ]; then
	sudo yum -y install java-1.7.0-openjdk
fi

# Install tools project (will ask for userid and password).
git clone https://github.com/alethix/tools.git
