#!/bin/sh

sudo yum -y update

# Install python.
#sudo yum -y install python

sudo yum -y install git

# Obtain the image scanner code.
sudo git clone https://github.com/baude/image-scanner.git
cd image-scanner

# Install and start docker.
# https://access.redhat.com/articles/881893#get
curl -sSL https://get.docker.com/ | sh
#sudo yum install docker-engine
sudo usermod -aG docker ec2-user
sudo service docker start

# Build the image scanner.
sudo docker build -t image-scanner -f docker/Dockerfile .

sudo yum -y install atomic

# Install image scanner image.
sudo atomic install image-scanner

# Run image.
sudo atomic run image-scanner
