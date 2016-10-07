# Clair Concepts
[Clair](https://github.com/coreos/clair) is a docker container image scanner that scans for known vulnerabilities.

Clair runs as a containerized REST service. One can use the REST API directly (I have), but the easiest way to use Clair is to use the small utility [analyze-local-images](https://github.com/coreos/clair/tree/master/contrib/analyze-local-images).
# Installing Clair
(*Pre-requisite:* The server must have the docker daemon and docker command line client installed, and it must have the go language installed - which is needed to install the clair client utility [analyze-local-images](https://github.com/coreos/clair/tree/master/contrib/analyze-local-images).)

The script `tools/Clair/deploy.sh` installs Clair on a server. The script `tools/create-tools-server.sh` creates a VM on which Clair can be installed, so the sequence is,
```
tools/create-tools-server.sh <parameters>
tools/Clair/deploy.sh <parameters>
```
The required parameters are documented in each script. The VM that is created is labeled with the name "DHS Demo - Tools". It has an elastic IP address so that the public IP address does not change across reboots - this allows us to access the tools server remotely, e.g., from a local script, if desired, although it appears that we will not be doing this.
# How To Scan a Container Image - Procedure
To scan an image, run the `scan-container-image.sh` script for the project pertaining to the image. This script in turn runs `clair_scan.sh` remotely on the Tools server (local IP address 172.31.20.21). If there is no `scan-container-image.sh`, you can run Clair against an image by doing this:
```
ssh -i <alethixkey> ec2-user@172.31.20.21 tools/Clair/clair_scan.sh hub.docker.com/alethix <registry-userid> <registry-pswd> <image-name> > <output-file-name>
```
E.g.,
```
ssh -i alethixkey.pem ec2-user@172.31.20.21 tools/Clair/clair_scan.sh hub.docker.com/alethix my-dockerhub-userid my-dockerhub-pswd ubuntu > report.txt
```
In this example, `image-name` would be the name of the container image that you want to scan. Note that `image-name` is a docker-compatible image name, which can have a tag (or not). E.g., the "name" of [our centos-node-hello image](https://hub.docker.com/r/alethixit/centos-node-hello/) is `hub.docker.com/alethixit/centos-node-hello`, but one can also specify it as `hub.docker.com/alethixit/centos-node-hello:latest`. The results are written to the `output-file-name`. The parameters `registry-userid` and `registry-pswd` are userid/password credentials for the registry server that contains the image - normally we would use the dockerhub account used by Jenkins.
