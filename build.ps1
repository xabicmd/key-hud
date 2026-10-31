# Builds key-hud.exe with a full rebuild (-a, no stale build cache) from a
# temporary clean copy of the sources, then writes the exe to the requested
# output path.
#
# Known startup panic: nil pointer dereference in
# CreateCoreWebView2ControllerCompleted (go-webview2). The real trigger is the
# process's mandatory Windows integrity label: any exe carrying a Low integrity
# label (for example inherited from a sandboxed tree such as C:\Users\neoho\www)
# makes CreateCoreWebView2Controller fail with 0x800700AA, and the jchv binding
# treats that positive HRESULT as success and dereferences a nil controller.
# Building from a clean directory does NOT remove that label; an exe placed in
# the tree inherits it. See the "Known limits" section of README.md.
param(
    [string]$Output = "key-hud.exe"
)
$ErrorActionPreference = "Stop"

$src = $PSScriptRoot
$outPath = Join-Path $src $Output
$buildDir = Join-Path ([System.IO.Path]::GetTempPath()) ("key-hud-build-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $buildDir | Out-Null

try {
    Copy-Item (Join-Path $src "cmd") $buildDir -Recurse
    Copy-Item (Join-Path $src "internal") $buildDir -Recurse
    Copy-Item (Join-Path $src "go.mod") $buildDir
    Copy-Item (Join-Path $src "go.sum") $buildDir

    Push-Location $buildDir
    try {
        go build -a -ldflags "-H windowsgui -s -w" -o $outPath ./cmd/key-hud
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    } finally {
        Pop-Location
    }

    $built = Get-Item $outPath
    Write-Host "Built $($built.FullName) ($($built.Length) bytes)"
} finally {
    Remove-Item $buildDir -Recurse -Force -ErrorAction SilentlyContinue
}
