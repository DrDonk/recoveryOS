#!/bin/bash
VERSION=$(<VERSION)
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building recoveryOS version $VERSION"
echo "Build date: $BUILD_DATE"
echo "Commit: $COMMIT"

LDFLAGS="-X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE -X main.Commit=$COMMIT"

mkdir -p build
cp -v README.md ./build
cp -v CHANGELOG.md ./build
cp -v LICENSE ./build
cp -v recovery_urls.txt ./build
cp -v boards.json ./build

# AMD64 builds
echo "Building AMD64 versions..."
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/windows/amd64/recoveryOS.exe recoveryOS.go
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/linux/amd64/recoveryOS recoveryOS.go
GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/macos/amd64/recoveryOS recoveryOS.go
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/windows/amd64/macrecovery.exe macrecovery.go
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/linux/amd64/macrecovery macrecovery.go
GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/macos/amd64/macrecovery macrecovery.go


# ARM64 builds
echo "Building ARM64 versions..."
GOOS=windows GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/windows/arm64/recoveryOS.exe recoveryOS.go
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/linux/arm64/recoveryOS recoveryOS.go
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/macos/arm64/recoveryOS recoveryOS.go
GOOS=windows GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/windows/arm64/macrecovery.exe macrecovery.go
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/linux/arm64/macrecovery macrecovery.go
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/macos/arm64/macrecovery macrecovery.go

# Build distribution zip file
rm -vf ./dist/recoveryOS-$VERSION.zip
rm -vrf ./dist/recoveryOS-$VERSION.sha256
7z a ./dist/recoveryOS-$VERSION.zip ./build/*
shasum -a 256 ./dist/recoveryOS-$VERSION.zip > ./dist/recoveryOS-$VERSION.sha256

echo "Build complete!"
