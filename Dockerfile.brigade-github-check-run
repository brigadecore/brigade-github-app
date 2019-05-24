FROM quay.io/deis/lightweight-docker-go:v0.6.0
ENV CGO_ENABLED=0
WORKDIR /go/src/github.com/brigadecore/brigade-github-app
COPY cmd/check-run cmd/check-run
COPY pkg/ pkg/
COPY vendor/ vendor/
RUN go build -o bin/check-run ./cmd/check-run

FROM scratch
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=0 /go/src/github.com/brigadecore/brigade-github-app/bin/check-run /usr/local/bin/check-run
CMD ["/usr/local/bin/check-run"]
