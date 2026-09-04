#!/bin/bash

echo " ---- Build Cosmos (multi-arch: armv6/armv7/386/riscv64/ppc64le) ----"

rm -rf build
mkdir -p build

cp src/update.go src/launcher/update.go

# build_cosmos <arch-suffix> <GOARCH> <GOARM> <ldflag-arch>
# Builds the main binary and the launcher for one architecture.
build_cosmos() {
    local suffix="$1" goarch="$2" goarm="$3" archvar="$4"

    if [ -n "$goarm" ]; then
        export GOARM="$goarm"
    else
        unset GOARM
    fi

    env GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildArch=$archvar" -o "build/cosmos-$suffix" src/*.go
    if [ $? -ne 0 ]; then
        echo "ERROR: failed to build cosmos-$suffix"
        exit 1
    fi

    env GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildArch=$archvar" -o "build/cosmos-launcher-$suffix" ./src/launcher/launcher.go ./src/launcher/update.go
    if [ $? -ne 0 ]; then
        echo "ERROR: failed to build cosmos-launcher-$suffix"
        exit 1
    fi

    # Compress the executables. UPX does not support every architecture
    # (e.g. riscv64), so only compress for architectures it handles.
    case "$suffix" in
        armv6|armv7|386|ppc64le)
            upx -9 "build/cosmos-$suffix"
            upx -9 "build/cosmos-launcher-$suffix"
            ;;
        *)
            echo "WARN: UPX does not support $suffix, skipping compression"
            ;;
    esac

    chmod +x "build/cosmos-$suffix"
    chmod +x "build/cosmos-launcher-$suffix"
}

build_cosmos armv6 arm 6 armv6
build_cosmos armv7 arm 7 armv7
build_cosmos 386 386 "" 386
build_cosmos riscv64 riscv64 "" riscv64
build_cosmos ppc64le ppc64le "" ppc64le

echo " ---- Build complete, copy assets ----"

cp start.sh build/start.sh

# Copy per-arch restic binaries into build/. The workflow downloads and
# renames these (e.g. restic-arm, restic-386, restic-riscv64, ...).
for r in restic-*; do
    cp "$r" build/ 2>/dev/null || true
done

chmod +x build/start.sh
for b in build/restic-*; do
    chmod +x "$b" 2>/dev/null || true
done

cp -r static build/
cp -r GeoLite2-Country.mmdb build/
# Copy per-arch nebula binaries into build/. The workflow downloads and
# renames these (e.g. nebula-armv7, nebula-armv7-cert, nebula-386, ...).
for nb in nebula-*; do
    cp "$nb" build/ 2>/dev/null || true
done
cp -r Logo.png build/
mkdir build/images
cp client/src/assets/images/icons/cosmos_gray.png build/cosmos_gray.png
cp client/src/assets/images/icons/cosmos_gray.png cosmos_gray.png
echo '{' > build/meta.json
cat package.json | grep -E '"version"' >> build/meta.json
echo '  "buildDate": "'`date`'",' >> build/meta.json
echo '  "built from": "'`hostname`'"' >> build/meta.json
echo '}' >> build/meta.json

echo " ---- copy complete ----"