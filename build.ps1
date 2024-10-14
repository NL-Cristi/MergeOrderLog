# Build for Linux
$version = git tag
Write-Host "Building $version for Linux..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags "-X main.version=${version}" -o .\release\MergeOrderLog main.go

# Build for Windows
Write-Host "Building $version for Windows..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags "-X main.version=${version}" -o .\release\MergeOrderLog.exe main.go

Write-Host "Builds completed."

