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
createbuildenv: $(DEPLOYSCRIPTDIR)/createbuildenv.sh

# Compile the SafeHarborServer code.
compile: compilego

# Build the SafeHarborServer container image and push it to the project registry.
build: $(DEPLOYSCRIPTDIR)/buildcontainer.sh

# Deploy a SafeHarborServer container, and all of the other containers that it needs.
deploy: $(DEPLOYSCRIPTDIR)/deploys.sh

# Deploy a SafeHarborServer container, with options set so that no other containers are needed.
deploystandalone: $(DEPLOYSCRIPTDIR)/deploystandalone.sh

# Remove compilation artifacts.
clean: cleango

# Steop all containers that were started by deploy.
stop: $(DEPLOYSCRIPTDIR)/stop.sh

# Remove artifacts that were created by deploy. This deletes database state!!!!
undeploy: $(DEPLOYSCRIPTDIR)/undeploy.sh

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
	
$(DEPLOYSCRIPTDIR)/createbuildenv.sh:
	source $(DEPLOYSCRIPTDIR)/createbuildenv.sh
	touch $(DEPLOYSCRIPTDIR)/createbuildenv.sh

$(DEPLOYSCRIPTDIR)/buildcontainer.sh: createbuildenv.sh compilego docs
	source $(DEPLOYSCRIPTDIR)/buildcontainer.sh
	touch $(DEPLOYSCRIPTDIR)/buildcontainer.sh

$(DEPLOYSCRIPTDIR)/deploy.sh: buildcontainer.sh
	source $(DEPLOYSCRIPTDIR)/deploy.sh
	touch $(DEPLOYSCRIPTDIR)/deploy.sh

$(DEPLOYSCRIPTDIR)/deploystandalone.sh: buildcontainer.sh
	source $(DEPLOYSCRIPTDIR)/deploystandalone.sh
	touch $(DEPLOYSCRIPTDIR)/deploystandalone.sh

cleango:
	rm -r -f $(BUILDDIR)/$(PACKAGENAME)

$(DEPLOYSCRIPTDIR)/stop.sh:
	source $(DEPLOYSCRIPTDIR)/stop.sh
	touch $(DEPLOYSCRIPTDIR)/stop.sh

$(DEPLOYSCRIPTDIR)/undeploy.sh:
	source $(DEPLOYSCRIPTDIR)/undeploy.sh
	touch $(DEPLOYSCRIPTDIR)/undeploy.sh

infotask:
	@echo "Makefile for $(PRODUCTNAME)"
