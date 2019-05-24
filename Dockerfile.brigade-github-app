FROM quay.io/deis/lightweight-docker-go:v0.6.0
ENV CGO_ENABLED=0
WORKDIR /go/src/github.com/brigadecore/brigade-github-app
COPY cmd/github-gateway cmd/github-gateway
COPY pkg/ pkg/
COPY vendor/ vendor/
RUN go build -o bin/github-gateway ./cmd/github-gateway

FROM scratch
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=0 /go/src/github.com/brigadecore/brigade-github-app/bin/github-gateway /usr/local/bin/github-gateway
CMD ["/usr/local/bin/github-gateway", "--logtostderr"]
