# scripts/build.sh
#!/bin/bash

set -e

echo "🏗️ Building Stream Server..."

# Clean previous builds
rm -rf build/

# Create build directory
mkdir -p build

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o build/stream-server-linux-amd64 ./cmd/server
GOOS=darwin GOARCH=amd64 go build -o build/stream-server-darwin-amd64 ./cmd/server
GOOS=windows GOARCH=amd64 go build -o build/stream-server-windows-amd64.exe ./cmd/server

echo "✅ Build complete! Binaries are in ./build/"

# Make Linux binary executable
chmod +x build/stream-server-linux-amd64

echo "📦 Creating release archive..."
cd build
tar -czf stream-server-release.tar.gz stream-server-*
echo "✅ Release archive created: build/stream-server-release.tar.gz"