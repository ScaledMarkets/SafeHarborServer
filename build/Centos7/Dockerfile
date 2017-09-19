# Build and publish as follows:
# cp ../../bin/safeharbor .

FROM docker.io/centos:7
RUN mkdir /safeharbor
ADD ["safeharbor", "/safeharbor/"]
ADD ["*.crt", "/safeharbor/"]
ADD ["*.conf", "/safeharbor/"]
ADD ["*.json", "/safeharbor/"]
WORKDIR /safeharbor/
#CMD ["/safeharbor/safeharbor", "-port", "6000", "-secretkey", "${RandomString}", "-host", "${SafeHarborPublicHostname}", "-toggleemail", "-stubs", "-noregistry", "-inmem", "-debug"]
