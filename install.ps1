# Installs the latest persadomapicli release on Windows (x86-64 or ARM64). No administrator rights needed.
#   irm https://raw.githubusercontent.com/persado/martechapi-cli/main/install.ps1 | iex
# The asset names must match `make dist`. Errors use throw, never exit: under iex, exit closes the user's shell.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # the progress bar slows Invoke-WebRequest in Windows PowerShell 5.1
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'persado/martechapi-cli'
    $bin = 'persadomapicli'
    $base = "https://github.com/$repo/releases/latest/download"
    $dest = Join-Path $env:LOCALAPPDATA "Programs\$bin"

    # 32-bit PowerShell on 64-bit Windows reports x86 and sets PROCESSOR_ARCHITEW6432 to the real architecture.
    $cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    $arch = switch ($cpu) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "No release build for Windows on $cpu. Build from source: https://github.com/$repo#build-from-source" }
    }
    $asset = "$bin-windows-$arch.exe"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $asset from the latest release of $repo ..."
        Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
        Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile (Join-Path $tmp 'SHA256SUMS')

        $expected = $null
        foreach ($line in Get-Content (Join-Path $tmp 'SHA256SUMS')) {
            $fields = $line -split '\s+'
            if ($fields.Count -ge 2 -and $fields[1] -eq $asset) { $expected = $fields[0] }
        }
        if (-not $expected) { throw "$asset is not listed in SHA256SUMS" }
        $actual = (Get-FileHash -Algorithm SHA256 -Path (Join-Path $tmp $asset)).Hash
        if ($actual -ne $expected) { throw "Checksum mismatch for $asset" }
        Write-Host 'Checksum verified.'

        New-Item -ItemType Directory -Force -Path $dest | Out-Null
        $exe = Join-Path $dest "$bin.exe"
        Move-Item -Force -Path (Join-Path $tmp $asset) -Destination $exe
        # Removes the "downloaded from the internet" mark that triggers SmartScreen.
        Unblock-File -Path $exe
    }
    finally {
        Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
    }

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dest) {
        $newPath = if ($userPath) { "$userPath;$dest" } else { $dest }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $dest to your user Path. Open a new terminal to use '$bin' from any directory."
    }
    if (($env:Path -split ';') -notcontains $dest) { $env:Path = "$env:Path;$dest" }

    Write-Host ''
    Write-Host "Installed $(& $exe version) to $exe"
}
