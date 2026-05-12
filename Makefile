.PHONY: all build test clean proto package-deb package-zip setup-local setup-remote

BINARY_TUNNELD=tunneld
BINARY_TUNNELCTL=tunnelctl
VERSION=0.1.0
DEB_PACKAGE=$(BINARY_TUNNELD)_$(VERSION)_amd64.deb
ZIP_PACKAGE=$(BINARY_TUNNELD)_$(VERSION)_linux_amd64.zip

all: build

setup-local:
	go mod edit -replace github.com/BrunoSilvaFreire/gographs=../graphs
	go mod tidy

setup-remote:
	go mod edit -dropreplace github.com/BrunoSilvaFreire/gographs
	go mod tidy

build:
	go build -o $(BINARY_TUNNELD) ./cmd/tunneld/main.go
	go build -o $(BINARY_TUNNELCTL) ./cmd/tunnelctl/main.go

test:
	go test ./...

clean:
	rm -f $(BINARY_TUNNELD) $(BINARY_TUNNELCTL)
	rm -rf pkg-build
	rm -f *.deb *.zip

proto:
	PATH="$(PATH):$(shell go env GOPATH)/bin" protoc --proto_path=api/v1 --go_out=pkg/api/v1 --go_opt=paths=source_relative --go-grpc_out=pkg/api/v1 --go-grpc_opt=paths=source_relative api/v1/tunnel.proto api/v1/spec.proto

package-deb: build
	rm -rf pkg-build
	mkdir -p pkg-build/DEBIAN
	mkdir -p pkg-build/usr/local/bin
	mkdir -p pkg-build/etc/tunneld
	mkdir -p pkg-build/lib/systemd/system
	mkdir -p pkg-build/usr/share/bash-completion/completions
	
	# Binaries
	cp $(BINARY_TUNNELD) pkg-build/usr/local/bin/
	cp $(BINARY_TUNNELCTL) pkg-build/usr/local/bin/
	
	# Bash completions
	./$(BINARY_TUNNELD) completion bash > pkg-build/usr/share/bash-completion/completions/$(BINARY_TUNNELD)
	./$(BINARY_TUNNELCTL) completion bash > pkg-build/usr/share/bash-completion/completions/$(BINARY_TUNNELCTL)

	# Config sample
	cp tunnels.yaml.sample pkg-build/etc/tunneld/tunnels.yaml.sample
	
	# Systemd unit
	cp tunneld.service.sample pkg-build/lib/systemd/system/tunneld.service
	
	# Control file
	echo "Package: $(BINARY_TUNNELD)" > pkg-build/DEBIAN/control
	echo "Version: $(VERSION)" >> pkg-build/DEBIAN/control
	echo "Section: base" >> pkg-build/DEBIAN/control
	echo "Priority: optional" >> pkg-build/DEBIAN/control
	echo "Architecture: amd64" >> pkg-build/DEBIAN/control
	echo "Maintainer: tunneld team" >> pkg-build/DEBIAN/control
	echo "Description: Persistent local tunnel supervisor daemon" >> pkg-build/DEBIAN/control
	
	dpkg-deb --build pkg-build $(DEB_PACKAGE)

package-zip: build
	rm -f $(ZIP_PACKAGE)
	zip $(ZIP_PACKAGE) $(BINARY_TUNNELD) $(BINARY_TUNNELCTL) tunnels.yaml.sample tunneld.service.sample
