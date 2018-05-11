SHELL:="/bin/bash"

.PHONY: build
build:
	go build -o bin/github-gateway ./cmd/...

test:
	go test ./pkg/...

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

.PHONY: redeploy
redeploy: test
redeploy: docker-build
redeploy: docker-push
redeploy:
	kubectl delete `kubectl get po -l app=github-app-test-brigade-github-app -o name`
	@echo Waiting for pod to start... (20 seconds)
	sleep 20
	kubectl logs -f `kubectl get po -l app=github-app-test-brigade-github-app -o name | tail -n 1 | sed 's/pod\///'`
