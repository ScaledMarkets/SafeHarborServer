# Makefile for building the Safe Harbor Server.
# This does not deploy any servers: it merely complies and packages the code.


PRODUCTNAME=Safe Harbor Server
ORG=Scaled Markets
VERSION=1.0
BUILD=1234
EXECNAME=SafeHarborServer
CesantaServerName = docker_auth
CesantaServerIPAddr = 127.0.0.1
LocalKeyPath = $(CesantaServerName).key
LocalPemPath = $(CesantaServerName).pem
LocalCertPath = $(CesantaServerName).crt

.DELETE_ON_ERROR:
.ONESHELL:
.SUFFIXES:
.DEFAULT_GOAL: all

SHELL = /bin/sh

CURDIR?=$(shell pwd)

#GO_LDFLAGS=-ldflags "-X `go list ./version`.Version $(VERSION)"

.PHONY: all compile authcert vm deploy clean info
.DEFAULT: all

src_dir = $(CURDIR)/src

build_dir = $(CURDIR)/../bin

GOPATH=$(CURDIR)

all: compile authcert

$(build_dir):
	mkdir $(build_dir)

$(build_dir)/$(EXECNAME): $(build_dir) $(src_dir)/main

compile: $(build_dir)/$(EXECNAME)
	go build -o $(build_dir)/$(EXECNAME) main

cacert:
	sudo openssl req -x509 -nodes -newkey rsa:2048 \
		-keyout scaledmarkets.key -out scaledmarkets.pem
	sudo openssl x509 -outform der -in scaledmarkets.pem -out scaledmarkets.crt

# This is needed because if a TLS server is identified by IP address, then that
# IP address must be defined in the cert as a Subject Alternative Name (SAN).
extfile: extfile.cnf
	(echo subjectAltName = IP:$(CesantaServerIPAddr)) > extfile.cnf


# https://stackoverflow.com/questions/22666163/golang-tls-with-selfsigned-certificate
# https://serverfault.com/questions/611120/failed-tls-handshake-does-not-contain-any-ip-sans
# https://github.com/elastic/logstash-forwarder/issues/221
# Note: On Mac, openssl.cnf is in /System/Library/OpenSSL
authcert: extfile scaledmarkets.key scaledmarkets.pem
	sudo openssl req -nodes -keyout $(LocalKeyPath) -out req.pem -newkey rsa:2048
	sudo openssl x509 -req -days 365 -in req.pem -out $(LocalPemPath) \
		-CA scaledmarkets.pem -CAkey scaledmarkets.key -CAcreateserial \
		-extfile extfile.cnf
	sudo openssl x509 -outform der -in $(LocalPemPath) -out $(LocalCertPath)
	sudo chmod 740 $(LocalKeyPath)
	sudo chmod 740 $(LocalPemPath)
	# use 'password' as PEM passphrase.

showcert:
	openssl x509 -in $(LocalPemPath) -noout -text

clean:
	rm -r -f $(build_dir)
	rm -r -f $(test_build_dir)

info:
	@echo "Makefile for $(PRODUCTNAME)"

