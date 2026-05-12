#!/bin/bash
set -e

ARCH=$(go env GOARCH)
VERSION=$(scripts/get_version.sh)
PACKAGE="tunneld_${VERSION}_${ARCH}.deb"

make package-deb GOARCH=$ARCH VERSION=$VERSION || exit 1
sudo apt purge tunneld -y || true
sudo apt install "./${PACKAGE}" -y || exit 2
sudo systemctl daemon-reload || exit 3
sudo systemctl restart tunneld.service || exit 4
