FROM docker.io/library/golang:1.22.0@sha256:ef61a20960397f4d44b0e729298bf02327ca94f1519239ddc6d91689615b1367 as builder
LABEL maintainer="maintainer@cilium.io"
ADD . /go/src/github.com/cilium/team-manager
WORKDIR /go/src/github.com/cilium/team-manager
RUN make team-manager
RUN strip team-manager

FROM docker.io/library/alpine:3.19.1@sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b as certs
RUN apk --update add ca-certificates

FROM scratch
LABEL maintainer="maintainer@cilium.io"
COPY --from=builder /go/src/github.com/cilium/team-manager/team-manager /usr/bin/team-manager
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/bin/team-manager"]
