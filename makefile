# Makefile for building the Safe Harbor Server.
# Testing is not done by this makefile - see separate project "TestSafeHarborServer".


# Names: -----------------------------------------------------------------------
PRODUCTNAME := Safe Harbor Server
ORG := Scaled Markets
VERSION := 1.0
BUILD := 1234
PACKAGENAME := safeharbor
EXECNAME := $(PACKAGENAME)


# Locations: -------------------------------------------------------------------
PROJECTROOT := $(shell pwd)
BUILDSCRIPTDIR := $(PROJECTROOT)/build/Centos7
SRCDIR := $(PROJECTROOT)/src
BUILDDIR := $(PROJECTROOT)/bin
STATUSDIR := $(PROJECTROOT)/status


# Tools: -----------------------------------------------------------------------
SHELL := /bin/sh


# Public Tasks: ----------------------------------------------------------------
.DEFAULT_GOAL: all
.DEFAULT: compilego
.PHONY: all compile clean info

# Setup from scratch, build, and deploy.
all: compile buildcontainer deploy

# Compile the SafeHarborServer code.
compile: compilego

# Compile with instrumentation for code coverage.
cover: covergo

# Remove compilation artifacts.
clean: cleango

# Provide a description of this makefile.
info: infotask


# Internal Tasks: --------------------------------------------------------------
.DELETE_ON_ERROR:
.ONESHELL:
.NOTPARALLEL:
.SUFFIXES:
.PHONY: compilego docs cleango infotask

$(BUILDDIR):
	mkdir $(BUILDDIR)

compilego: $(BUILDDIR)
	@GOPATH=$(PROJECTROOT) go install $(PACKAGENAME)

# See https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests
# See https://blog.golang.org/cover
covergo: $(BUILDDIR)
	@GOPATH=$(PROJECTROOT) go test -c -o $(BUILDDIR)/safeharbor.test \
		-covermode count -coverpkg $(PACKAGENAME)/...

# Generate REST docs.
# http://apidocjs.com/
# https://howtonode.org/introduction-to-npm
docs: compilego
	
cleango:
	rm -r -f $(BUILDDIR)/$(PACKAGENAME)

infotask:
	@echo "Makefile for $(PRODUCTNAME)"
