# Build for Linux
Write-Host "Building for Linux..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o  .\release\MergeOrderLog main.go

# Build for Windows
Write-Host "Building for Windows..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o  .\release\MergeOrderLog.exe main.go

Write-Host "Builds completed."
