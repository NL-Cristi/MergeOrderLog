$VER = git describe --tags --long --always
if ($VER -match '^([^-]+)-(\d+)-(g[0-9a-f]+)$') {
    $tag  = $Matches[1]
    $hash = $Matches[3]
    $VER  = "$tag($hash)"
}

# Build for Linux
Write-Host "Building $VER for Linux..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags "-X main.version=${VER}" -o .\release\MergeOrderLog main.go

# Build for Windows
Write-Host "Building $VER for Windows..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags "-X main.version=${VER}" -o .\release\MergeOrderLog.exe main.go

Write-Host "Builds completed."

