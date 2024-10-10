# Build for Linux
Write-Host "Building for Linux..."
go build -o MergeOrderLog-linux -GOOS linux -GOARCH amd64 main.go

# Build for Windows
Write-Host "Building for Windows..."
go build -o MergeOrderLog.exe -GOOS windows -GOARCH amd64 main.go

Write-Host "Builds completed."


