.PHONY: all build test clean proto package-deb package-zip setup-local setup-remote

BINARY_TUNNELD=tunneld
BINARY_TUNNELCTL=tunnelctl
VERSION ?= $(shell scripts/get_version.sh)
GOARCH ?= $(shell go env GOARCH)
DEB_PACKAGE=$(BINARY_TUNNELD)_$(VERSION)_$(GOARCH).deb
ZIP_PACKAGE=$(BINARY_TUNNELD)_$(VERSION)_linux_$(GOARCH).zip

all: build

setup-local:
	go mod edit -replace github.com/BrunoSilvaFreire/gographs=../graphs
	go mod tidy

setup-remote:
	go mod edit -dropreplace github.com/BrunoSilvaFreire/gographs
	go mod tidy

build:
	GOARCH=$(GOARCH) go build -o $(BINARY_TUNNELD) ./cmd/tunneld/main.go
	GOARCH=$(GOARCH) go build -o $(BINARY_TUNNELCTL) ./cmd/tunnelctl/main.go

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
	mkdir -p pkg-build/lib/systemd/system
	@echo "[Unit]" > pkg-build/lib/systemd/system/tunneld.service
	@echo "Description=Tunnel supervisor" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "After=network-online.target" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "Wants=network-online.target" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "[Service]" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "User=tunneld" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "Group=tunneld" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "ExecStart=/usr/local/bin/tunneld --config /etc/tunneld/tunnels.yaml run" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "Restart=always" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "RestartSec=3" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "KillSignal=SIGTERM" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "TimeoutStopSec=30" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "[Install]" >> pkg-build/lib/systemd/system/tunneld.service
	@echo "WantedBy=multi-user.target" >> pkg-build/lib/systemd/system/tunneld.service

	# Control file
	echo "Package: $(BINARY_TUNNELD)" > pkg-build/DEBIAN/control
	echo "Version: $(VERSION)" >> pkg-build/DEBIAN/control
	echo "Section: base" >> pkg-build/DEBIAN/control
	echo "Priority: optional" >> pkg-build/DEBIAN/control
	echo "Architecture: $(GOARCH)" >> pkg-build/DEBIAN/control
	echo "Maintainer: tunneld team" >> pkg-build/DEBIAN/control
	echo "Description: Persistent local tunnel supervisor daemon" >> pkg-build/DEBIAN/control
	
	# Postinst script
	echo "#!/bin/sh" > pkg-build/DEBIAN/postinst
	echo "set -e" >> pkg-build/DEBIAN/postinst
	echo "if ! getent group tunneld >/dev/null; then" >> pkg-build/DEBIAN/postinst
	echo "    addgroup --system tunneld" >> pkg-build/DEBIAN/postinst
	echo "fi" >> pkg-build/DEBIAN/postinst
	echo "if ! getent passwd tunneld >/dev/null; then" >> pkg-build/DEBIAN/postinst
	echo "    adduser --system --ingroup tunneld --no-create-home --home /etc/tunneld --disabled-password --disabled-login tunneld" >> pkg-build/DEBIAN/postinst
	echo "fi" >> pkg-build/DEBIAN/postinst
	echo "mkdir -p /var/lib/tunneld/keys" >> pkg-build/DEBIAN/postinst
	echo "chown -R tunneld:tunneld /etc/tunneld" >> pkg-build/DEBIAN/postinst
	echo "chown -R tunneld:tunneld /var/lib/tunneld" >> pkg-build/DEBIAN/postinst
	echo "chmod 700 /var/lib/tunneld" >> pkg-build/DEBIAN/postinst
	chmod 755 pkg-build/DEBIAN/postinst

	dpkg-deb --build pkg-build $(DEB_PACKAGE)

package-zip: build
	rm -f $(ZIP_PACKAGE)
	zip $(ZIP_PACKAGE) $(BINARY_TUNNELD) $(BINARY_TUNNELCTL) tunnels.yaml.sample
