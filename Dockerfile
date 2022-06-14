FROM docker.io/library/golang:1.18.1@sha256:12d3995156cb0dcdbb9d3edb5827e4e8e1bf5bf92436bfd12d696ec997001a9a as builder
LABEL maintainer="maintainer@cilium.io"
ADD . /go/src/github.com/cilium/team-manager
WORKDIR /go/src/github.com/cilium/team-manager
ARG TARGETARCH
RUN make team-manager
RUN strip team-manager

FROM docker.io/library/alpine:3.12.7@sha256:36553b10a4947067b9fbb7d532951066293a68eae893beba1d9235f7d11a20ad as certs
RUN apk --update add ca-certificates

FROM scratch
LABEL maintainer="maintainer@cilium.io"
COPY --from=builder /go/src/github.com/cilium/team-manager/team-manager /usr/bin/team-manager
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/bin/team-manager"]
