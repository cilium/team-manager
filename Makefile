VERSION=$(shell git describe --tags --always)

.PHONY: all
all: local test

.PHONY: docker-image
docker-image:
	docker build -t cilium/team-manager:${VERSION} .

.PHONY: test
test:
	go test -mod=vendor ./...

.PHONY: team-manager
team-manager:
	CGO_ENABLED=0 go build -mod=vendor -a -installsuffix cgo -o $@ ./cmd/

.PHONY: local
local: team-manager
	strip team-manager

.PHONY: clean
clean:
	rm -fr team-manager
