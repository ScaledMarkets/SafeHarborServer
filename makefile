# Makefile for compiling the Safe Harbor Server.
# Testing is not done by this makefile - see separate project "TestSafeHarborServer".

# To do: Incorporate https://github.com/awslabs/git-secrets


# Names: -----------------------------------------------------------------------
PRODUCTNAME := Safe Harbor Server
ORG := Scaled Markets
VERSION := 1.0
BUILD := 1234
PACKAGENAME := safeharbor
EXECNAME := $(PACKAGENAME)
CPU_ARCH:=$(shell uname -s | tr '[:upper:]' '[:lower:]')_amd64


# Locations: -------------------------------------------------------------------
PROJECTROOT := $(shell pwd)
BUILDSCRIPTDIR := $(PROJECTROOT)/build/Centos7
SRCDIR := $(PROJECTROOT)/src
BUILDDIR := $(PROJECTROOT)/bin
PKGDIR := $(PROJECTROOT)/pkg
STATUSDIR := $(PROJECTROOT)/status
UTILITIESDIR:=$(realpath $(PROJECTROOT)/../utilities/utils)
RESTDIR:=$(realpath $(PROJECTROOT)/../utilities/rest)
SCANNERSDIR:=$(realpath $(PROJECTROOT)/../scanners)
DOCKERDIR:=$(realpath $(PROJECTROOT)/../docker)

# Tools: -----------------------------------------------------------------------
SHELL := /bin/sh


# Tasks: ----------------------------------------------------------------

.DEFAULT_GOAL: all
.DEFAULT: compilego
.PHONY: all compile clean info
.DELETE_ON_ERROR:
.ONESHELL:
.NOTPARALLEL:
.SUFFIXES:
.PHONY: compile cover docs clean info

$(BUILDDIR):
	mkdir $(BUILDDIR)

# Main executable depends on source files.
$(BUILDDIR)/$(EXECNAME): $(BUILDDIR) $(SRCDIR)/$(PACKAGENAME)/*.go

# The compile target depends on the main executable.
# 'make compile' builds the executable, which is placed in <build_dir>.
compile: $(BUILDDIR)/$(EXECNAME)
	GOPATH=$(PROJECTROOT):$(SCANNERSDIR):$(DOCKERDIR):$(UTILITIESDIR):$(RESTDIR) go install $(PACKAGENAME)

# See https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests
# See https://blog.golang.org/cover
cover: $(BUILDDIR)
	GOPATH=$(PROJECTROOT) go test -c -o $(BUILDDIR)/safeharbor.test \
		-covermode count -coverpkg $(PACKAGENAME)/...

# Generate REST docs.
# http://apidocjs.com/
# https://howtonode.org/introduction-to-npm
docs: compile
	
clean:
	rm -r -f $(BUILDDIR)/$(PACKAGENAME)

info:
	@echo "Makefile for $(PRODUCTNAME)"
