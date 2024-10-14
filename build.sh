#!/bin/bash
version=$(git tag)
# Build for Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${version}" -o ./release/MergeOrderLog main.go

# Build for Windows
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=${version}" -o ./release/MergeOrderLog.exe main.go

echo "Builds completed."
