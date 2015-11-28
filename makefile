# Makefile for building the Safe Harbor Server.
# This does not deploy any servers: it merely complies and packages the code.
# Testing is not done by this makefile - see separate project "TestSafeHarborServer".


PRODUCTNAME=Safe Harbor Server
ORG=Scaled Markets
VERSION=1.0
BUILD=1234
EXECNAME=SafeHarborServer

.DELETE_ON_ERROR:
.ONESHELL:
.SUFFIXES:
.DEFAULT_GOAL: all

SHELL = /bin/sh

CURDIR=$(shell pwd)

#GO_LDFLAGS=-ldflags "-X `go list ./version`.Version $(VERSION)"

.PHONY: all compile authcert vm deploy clean info
.DEFAULT: all

src_dir = $(CURDIR)/src

build_dir = $(CURDIR)/../bin

#GOPATH = $(CURDIR)/..
#GO_LDFLAGS=-ldflags "-X `go list ./version`.Version $(VERSION)"

all: compile authcert

$(build_dir):
	mkdir $(build_dir)

$(build_dir)/$(EXECNAME): $(build_dir) $(src_dir)

# 'make compile' builds the executable, which is placed in <build_dir>.
compile: $(build_dir)/$(EXECNAME)
	@echo GOPATH=$(GOPATH)
	@GOPATH=$(CURDIR) go build ./src/...
	#@GOPATH=$(CURDIR) go build -tags "${DOCKER_BUILDTAGS}" -v ${GO_LDFLAGS} ./src/...
	#GOPATH=$(CURDIR) go install main
	#go build -o $(build_dir)/$(EXECNAME) main rest

clean:
	rm -r -f $(build_dir)
	rm -r -f $(test_build_dir)

info:
	@echo "Makefile for $(PRODUCTNAME)"

