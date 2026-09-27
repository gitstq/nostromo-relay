#!/usr/bin/env sh
# One-shot cross-platform build for Nostromo.
# Usage: ./scripts/build.sh [version]
set -e

VERSION="${1:-1.0.0}"
MODULE="github.com/gitstq/nostromo-relay"
OUT="bin/release"
LDFLAGS="-s -w -X ${MODULE}/internal/server.Version=${VERSION}"

mkdir -p "$OUT"

build() {
	os="$1"; arch="$2"
	target="$OUT/nostromo_${os}_${arch}"
	if [ "$os" = "windows" ]; then target="${target}.exe"; fi
	echo "-> $target"
	GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath \
		-ldflags="$LDFLAGS" -o "$target" ./cmd/nostromo
}

for arch in amd64 arm64; do
	build linux "$arch"
	build darwin "$arch"
	build windows "$arch"
done

# Checksums (use sha256sum when available, otherwise shasum).
cd "$OUT"
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum * > checksums.txt
else
	shasum -a 256 * > checksums.txt
fi
echo "done. artifacts in $OUT"
cat checksums.txt
