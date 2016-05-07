# Makefile for building and deploying the Safe Harbor Server.
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
DEPLOYSCRIPTDIR := $(PROJECTROOT)/deploy/AWS
SRCDIR := $(PROJECTROOT)/src
BUILDDIR := $(PROJECTROOT)/bin
STATUSDIR := $(PROJECTROOT)/status


# Tools: -----------------------------------------------------------------------
SHELL := /bin/sh


# Public Tasks: ----------------------------------------------------------------
# All tasks assume that the docker daemon is running.
.DEFAULT_GOAL: all
.DEFAULT: compilego
.PHONY: all createbuildenv compile build deploy deploystandalone clean stop undeploy info

# Setup from scratch, build, and deploy.
all: compile createbuildenv buildcontainer deploy

# Install go and git, and clone the SafeHarborServer repo.
createbuildenv: createbuildenv.sh

# Compile the SafeHarborServer code.
compile: compilego

# Build the SafeHarborServer container image and push it to the project registry.
build: buildcontainer.sh

# Deploy a SafeHarborServer container, and all of the other containers that it needs.
deploy: deploys.sh

# Deploy a SafeHarborServer container, with options set so that no other containers are needed.
deploystandalone: deploystandalone.sh

# Remove compilation artifacts.
clean: cleango

# Steop all containers that were started by deploy.
stop: stop.sh

# Remove artifacts that were created by deploy. This deletes database state!!!!
undeploy: undeploy.sh

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

# 'make compilego' builds the executable, which is placed in <BUILDDIR>.
compilego: $(BUILDDIR)
	@GOPATH=$(PROJECTROOT) go install $(PACKAGENAME)

# Generate REST docs.
# http://apidocjs.com/
# https://howtonode.org/introduction-to-npm
docs: compilego
	
createbuildenv.sh:
	source $(DEPLOYSCRIPTDIR)/createbuildenv.sh
	touch createbuildenv.sh

buildcontainer.sh: createbuildenv.sh compilego docs
	source $(DEPLOYSCRIPTDIR)/buildcontainer.sh
	touch buildcontainer.sh

deploy.sh: buildcontainer.sh
	source $(DEPLOYSCRIPTDIR)/deploy.sh
	touch deploy.sh

deploystandalone.sh: buildcontainer.sh
	source deploystandalone.sh
	touch deploystandalone.sh

cleango:
	rm -r -f $(BUILDDIR)/$(PACKAGENAME)

stop.sh:
	source stop.sh
	touch stop.sh

undeploy.sh:
	source undeploy.sh
	touch undeploy.sh

infotask:
	@echo "Makefile for $(PRODUCTNAME)"
