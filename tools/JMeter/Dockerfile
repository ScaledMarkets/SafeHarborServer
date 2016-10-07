# Build a JMeter performance tool container.
# Ref: http://stackoverflow.com/questions/36698220/jmeter-docker-java-net-connectexception
FROM docker.io/centos:7
RUN yum update
RUN yum -y install wget
RUN yum -y install default-jre
RUN mkdir /jmeter
RUN cd /jmeter/ ; wget http://www.us.apache.org/dist//jmeter/binaries/apache-jmeter-2.13.tgz ; tar -xzf apache-jmeter-2.13.tgz ; rm apache-jmeter-2.13.tgz
ENV JMETER_HOME /jmeter/apache-jmeter-2.13/
ENV PATH $JMETER_HOME/bin:$PATH
EXPOSE 1099 7000 50000
ENTRYPOINT $JMETER_HOME/bin/jmeter-server -Djava.rmi.server.hostname=$LOCALIP
