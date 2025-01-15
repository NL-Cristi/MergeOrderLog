#!/bin/bash

# Pull an extended description: e.g. v1.0.2-16-g9413b85
version=$(git describe --tags --long --always 2>/dev/null)

# If the output matches <tag>-<commits>-<hash>, transform it to <tag>(<hash>)
if [[ $version =~ ^([^-]+)-([0-9]+)-(g[0-9a-f]+)$ ]]; then
  tag="${BASH_REMATCH[1]}"
  hash="${BASH_REMATCH[3]}"
  version="${tag}(${hash})"
fi

# Build for Linux
echo "Building $version for Linux..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${version}" -o ./release/MergeOrderLog main.go

# Build for Windows
echo "Building $version for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=${version}" -o ./release/MergeOrderLog.exe main.go

echo "Builds completed."
