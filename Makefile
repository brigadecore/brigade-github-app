SHELL ?= /bin/bash

.DEFAULT_GOAL := build

################################################################################
# Version details                                                              #
################################################################################

# This will reliably return the short SHA1 of HEAD or, if the working directory
# is dirty, will return that + "-dirty"
GIT_VERSION = $(shell git describe --always --abbrev=7 --dirty --match=NeVeRmAtCh)

################################################################################
# Go build details                                                             #
################################################################################

BASE_PACKAGE_NAME := github.com/brigadecore/brigade-github-app

################################################################################
# Containerized development environment-- or lack thereof                      #
################################################################################

ifneq ($(SKIP_DOCKER),true)
	PROJECT_ROOT := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
	DEV_IMAGE := krancour/go-tools:v0.1.0
	DOCKER_CMD := docker run \
		-it \
		--rm \
		-e SKIP_DOCKER=true \
		-v $(PROJECT_ROOT):/go/src/$(BASE_PACKAGE_NAME) \
		-w /go/src/$(BASE_PACKAGE_NAME) $(DEV_IMAGE)
endif

################################################################################
# Docker images we build and publish                                           #
################################################################################

ifdef DOCKER_REGISTRY
	DOCKER_REGISTRY := $(DOCKER_REGISTRY)/
endif

ifdef DOCKER_ORG
	DOCKER_ORG := $(DOCKER_ORG)/
endif

DOCKER_IMAGE_PREFIX := $(DOCKER_REGISTRY)$(DOCKER_ORG)

ifdef VERSION
	IMMUTABLE_DOCKER_TAG := $(VERSION)
	MUTABLE_DOCKER_TAG   := latest
else
	IMMUTABLE_DOCKER_TAG := $(GIT_VERSION)
	MUTABLE_DOCKER_TAG   := edge
endif

################################################################################
# Utility targets                                                              #
################################################################################

.PHONY: redeploy
redeploy: test push-all-images
redeploy:
	kubectl delete `kubectl get po -l app=github-app-test-brigade-github-app -o name`
	@echo 'Waiting for pod to start... (20 seconds)'
	sleep 20
	kubectl logs -f `kubectl get po -l app=github-app-test-brigade-github-app -o name | tail -n 1 | sed 's/pod\///'`

.PHONY: dep
dep:
	$(DOCKER_CMD) go mod tidy && go mod vendor

################################################################################
# Tests                                                                        #
################################################################################

# Verifies there are no discrepancies between desired dependencies and the
# tracked, vendored dependencies
.PHONY: verify-vendored-code
verify-vendored-code:
	$(DOCKER_CMD) go mod verify

.PHONY: lint
lint:
	$(DOCKER_CMD) golangci-lint run --config ./golangci.yml

.PHONY: test
test:
	$(DOCKER_CMD) go test ./pkg/...

################################################################################
# Build / Publish                                                              #
################################################################################

IMAGES = brigade-github-app brigade-github-check-run

.PHONY: build
build: build-all-images

# To use build-all-images, you need to have Docker installed and configured. You
# should also set DOCKER_REGISTRY and DOCKER_ORG to your own personal registry
# if you are not pushing to the official upstream.
.PHONY: build-all-images
build-all-images: $(addsuffix -build-image,$(IMAGES))

%-build-image:
	docker build -f Dockerfile.$* -t $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG)

.PHONY: push
push: push-all-images

# You must be logged into DOCKER_REGISTRY before you can push.
.PHONY: push-all-images
push-all-images: build-all-images
push-all-images: $(addsuffix -push-image,$(IMAGES))

%-push-image:
	docker push $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG)
	docker push $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG)
