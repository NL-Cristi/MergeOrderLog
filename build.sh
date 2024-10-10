#!/bin/bash

# Build for Linux
echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o MergeOrderLog-linux main.go

# Build for Windows
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o MergeOrderLog.exe main.go

echo "Builds completed."
