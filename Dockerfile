FROM alpine:3.7
EXPOSE 8080

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY rootfs/github-gateway /usr/local/bin/github-gateway

CMD /usr/local/bin/github-gateway
