# Builds the portable, self-contained Windows 11 x64 ZIP.
# Run on Windows only; the native app and bundled Go child are Windows binaries.
$ErrorActionPreference = "Stop"
$root = Resolve-Path "$PSScriptRoot/.."
$out = Join-Path $root "artifacts/windows"
New-Item -ItemType Directory -Force -Path $out | Out-Null

Push-Location $root
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -trimpath -ldflags="-s -w" -o "$out/f1tele.exe" ./server/cmd/f1tele
Pop-Location

Push-Location "$root/race_engineer_hub"
npm ci
npm run build
Pop-Location
Copy-Item -Recurse -Force "$root/race_engineer_hub/dist/*" "$root/server/internal/web/dist/"
Push-Location $root
go build -trimpath -ldflags="-s -w" -o "$out/f1tele.exe" ./server/cmd/f1tele
Pop-Location

dotnet publish "$root/windows/F1Telemetry.ControlCenter" -c Release -r win-x64 --self-contained true -o "$out/app"
Copy-Item "$out/f1tele.exe" "$out/app/f1tele.exe"
$zip = Join-Path $root "artifacts/F1Telemetry-1.0.0-win-x64.zip"
if (Test-Path $zip) { Remove-Item $zip }
Compress-Archive -Path "$out/app/*" -DestinationPath $zip
Write-Host "Wrote $zip"
