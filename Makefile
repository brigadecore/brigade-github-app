SHELL ?= /bin/bash

# The Docker registry where images are pushed.
# Note that if you use an org (like on Quay and DockerHub), you should
# include that: quay.io/foo
DOCKER_REGISTRY    ?= deis

GIT_TAG   = $(shell git describe --tags --always 2>/dev/null)
VERSION   ?= ${GIT_TAG}
IMAGE_TAG ?= ${VERSION}

BINS = github-gateway check-run
IMAGES = brigade-github-app brigade-github-check-run

.PHONY: build
build: $(BINS)

.PHONY: $(BINS)
$(BINS): bootstrap
	go build -o bin/$@ ./cmd/$@

# Cross-compile for Docker+Linux
.PHONY: build-docker-bins
build-docker-bins: $(addsuffix -docker-bin,$(BINS))

%-docker-bin: bootstrap
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o ./rootfs/$* ./cmd/$*

# To use docker-build, you need to have Docker installed and configured. You should also set
# DOCKER_REGISTRY to your own personal registry if you are not pushing to the official upstream.
.PHONY: docker-build
docker-build: build-docker-bins
docker-build: $(addsuffix -image,$(IMAGES))

%-image:
	docker build -f Dockerfile.$* -t $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG) .

# You must be logged into DOCKER_REGISTRY before you can push.
.PHONY: docker-push
docker-push: $(addsuffix -push,$(IMAGES))

%-push:
	docker push $(DOCKER_REGISTRY)/$*:$(IMAGE_TAG)

.PHONY: lint
lint:
	golangci-lint run --config ./golangci.yml

.PHONY: test
test:
	go test ./pkg/...

.PHONY: redeploy
redeploy: test
redeploy: docker-build
redeploy: docker-push
redeploy:
	kubectl delete `kubectl get po -l app=github-app-test-brigade-github-app -o name`
	@echo 'Waiting for pod to start... (20 seconds)'
	sleep 20
	kubectl logs -f `kubectl get po -l app=github-app-test-brigade-github-app -o name | tail -n 1 | sed 's/pod\///'`

HAS_DEP          := $(shell command -v dep;)
HAS_GOLANGCI     := $(shell command -v golangci-lint;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin
endif
	dep ensure