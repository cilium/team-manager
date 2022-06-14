BUILD_VARS=CGO_ENABLED=0 GOARCH=${TARGETARCH}
IMAGE=cilium/team-manager
VERSION=latest

all: local

docker-image:
	docker buildx build --push --builder default -t $(IMAGE):$(VERSION) .

tests:
	$(BUILD_VARS) go test -mod=vendor ./...

team-manager: tests
	$(BUILD_VARS) go build -mod=vendor -a -installsuffix cgo -o $@ ./cmd/main.go

local: team-manager
	strip team-manager

clean:
	rm -fr team-manager
