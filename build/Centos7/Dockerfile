# Build and publish as follows:
# cp ../../bin/safeharbor .
# docker build -t scaledmarkets/safeharborserver .
# docker tag scaledmarkets/safeharborserver:latest 500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets:latest
# docker push 500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets/safeharborserver

FROM docker.io/centos:7
RUN mkdir /safeharbor
ADD ["safeharbor", "/safeharbor/safeharbor"]
ADD ["*.yaml", "/safeharbor/"]
ADD ["*.conf", "/safeharbor/"]
ADD ["*.json", "/safeharbor/"]
ENTRYPOINT ["/safeharbor/safeharbor"]
CMD ["/safeharbor/safeharbor", "-port", "6000"]
