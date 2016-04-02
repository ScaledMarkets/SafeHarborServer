# Build and publish as follows:
# cp ../../bin/safeharbor .
# docker build -t scaledmarkets/safeharborserver .
# docker tag scaledmarkets/safeharborserver:latest 500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets:latest
# docker push 500058573678.dkr.ecr.us-east-1.amazonaws.com/scaledmarkets/safeharborserver

FROM dockerhub/centos:7
WORKDIR /
ADD ["safeharbor", "."]
ADD ["*.yaml", "."]
ADD ["*.conf", "."]
ADD ["*.json", "."]
ENTRYPOINT ["safeharbor"]
CMD ["-port", "6000"]
