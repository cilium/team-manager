FROM docker.io/library/golang:1.20.5@sha256:344193a70dc3588452ea39b4a1e465a8d3c91f788ae053f7ee168cebf18e0a50 as builder
LABEL maintainer="maintainer@cilium.io"
ADD . /go/src/github.com/cilium/team-manager
WORKDIR /go/src/github.com/cilium/team-manager
RUN make team-manager
RUN strip team-manager

FROM docker.io/library/alpine:3.18.2@sha256:82d1e9d7ed48a7523bdebc18cf6290bdb97b82302a8a9c27d4fe885949ea94d1 as certs
RUN apk --update add ca-certificates

FROM scratch
LABEL maintainer="maintainer@cilium.io"
COPY --from=builder /go/src/github.com/cilium/team-manager/team-manager /usr/bin/team-manager
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/bin/team-manager"]
