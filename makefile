# Makefile for building the Safe Harbor Server.
# This does not deploy any servers: it merely complies and packages the code.

PRODUCTNAME=Safe Harbor Server
ORG=Scaled Markets
VERSION=1.0
BUILD=1234
EXECNAME=SafeHarborServer
CesantaServerName = docker_auth
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

.PHONY: all compile vm deploy clean info
.DEFAULT: all

GOPATH=$(CURDIR)

src_dir = $(CURDIR)/src

build_dir = $(CURDIR)/../bin

all: compile authcert

$(build_dir):
	mkdir $(build_dir)

$(build_dir)/$(EXECNAME): $(build_dir) $(src_dir)/main

compile: $(build_dir)/$(EXECNAME)
	go build -o $(build_dir)/$(EXECNAME) main

authcert:
	sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout $(LocalKeyPath) -out $(LocalPemPath)
	sudo openssl x509 -outform der -in $(LocalPemPath) -out $(LocalCertPath)
	sudo chmod 740 $(LocalKeyPath)
	sudo chmod 740 $(LocalPemPath)

clean:
	rm -r -f $(build_dir)
	rm -r -f $(test_build_dir)

info:
	@echo "Makefile for $(PRODUCTNAME)"

