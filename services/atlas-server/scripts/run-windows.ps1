$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
if (-not (Test-Path "atlas-server.toml")) {
  Copy-Item "atlas-server.toml.example" "atlas-server.toml"
  Write-Host "Created atlas-server.toml from example — edit MySQL settings before production use."
}
# Optional env overrides still work: $env:ATLAS_MYSQL_DSN = "..."
go run ./cmd/server
