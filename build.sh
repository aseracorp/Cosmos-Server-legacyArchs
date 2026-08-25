#!/bin/bash

echo " ---- Build Cosmos (legacy archs: armv6/armv7) ----"

rm -rf build

cp src/update.go src/launcher/update.go

env GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildGOARM=6" -o build/cosmos-armv6 src/*.go
if [ $? -ne 0 ]; then
    exit 1
fi
env GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildGOARM=6" -o build/cosmos-launcher-armv6 ./src/launcher/launcher.go ./src/launcher/update.go
if [ $? -ne 0 ]; then
    exit 1
fi

env GOARCH=arm CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildGOARM=7" -o build/cosmos-armv7 src/*.go
if [ $? -ne 0 ]; then
    exit 1
fi
env GOARCH=arm CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.BuildGOARM=7" -o build/cosmos-launcher-armv7 ./src/launcher/launcher.go ./src/launcher/update.go
if [ $? -ne 0 ]; then
    exit 1
fi

# Compress the executable (test performance impact before using in production).
upx -9 build/cosmos-armv6
upx -9 build/cosmos-launcher-armv6
upx -9 build/cosmos-armv7
upx -9 build/cosmos-launcher-armv7

echo " ---- Build complete, copy assets ----"

cp start.sh build/start.sh
cp restic-arm build/

chmod +x build/start.sh
chmod +x build/cosmos-armv6
chmod +x build/cosmos-launcher-armv6
chmod +x build/cosmos-armv7
chmod +x build/cosmos-launcher-armv7
chmod +x build/restic
chmod +x build/restic-arm

cp -r static build/
cp -r GeoLite2-Country.mmdb build/
cp nebula-armv6-cert nebula-armv7-cert nebula-cert nebula-armv6 nebula-armv7 nebula build/
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
