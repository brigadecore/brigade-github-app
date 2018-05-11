
.PHONY: build
build:
	go build -o bin/github-gateway ./cmd/...

# To use docker-build, you need to have Docker installed and configured. You should also set
# DOCKER_REGISTRY to your own personal registry if you are not pushing to the official upstream.
.PHONY: docker-build
docker-build:
	GOOS=linux GOARCH=amd64 go build -o rootfs/github-gateway ./cmd/...
	docker build -t technosophos/brigade-github-app:latest .

# You must be logged into DOCKER_REGISTRY before you can push.
.PHONY: docker-push
docker-push:
	docker push technosophos/brigade-github-app
