SHELL:=/bin/bash

.PHONY: build
build:
	go build -o bin/github-gateway ./cmd/github-gateway/...
	go build -o bin/check-run ./cmd/check-run...

test:
	go test ./pkg/...

# To use docker-build, you need to have Docker installed and configured. You should also set
# DOCKER_REGISTRY to your own personal registry if you are not pushing to the official upstream.
.PHONY: docker-build
docker-build: docker-build-gateway
docker-build: docker-build-check-run

.PHONY: docker-build-gateway
docker-build-gateway:
	GOOS=linux GOARCH=amd64 go build -o rootfs/github-gateway ./cmd/github-gateway/...
	docker build -t deis/brigade-github-app:latest .

.PHONY: docker-build-check-run
docker-build-check-run:
	GOOS=linux GOARCH=amd64 go build -o rootfs/check-run ./cmd/check-run/...
	docker build -f Dockerfile.check-run -t deis/brigade-github-check-run:latest .

# You must be logged into DOCKER_REGISTRY before you can push.
.PHONY: docker-push
docker-push:
	docker push deis/brigade-github-app
	docker push deis/brigade-github-check-run

.PHONY: redeploy
redeploy: test
redeploy: docker-build
redeploy: docker-push
redeploy:
	kubectl delete `kubectl get po -l app=github-app-test-brigade-github-app -o name`
	@echo 'Waiting for pod to start... (20 seconds)'
	sleep 20
	kubectl logs -f `kubectl get po -l app=github-app-test-brigade-github-app -o name | tail -n 1 | sed 's/pod\///'`

.PHONY: bootstrap
bootstrap:
	dep ensure