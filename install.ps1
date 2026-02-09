# Claude Squad Windows Installation Script
# PowerShell version of install.sh

param(
    [string]$Name = "cs",
    [string]$Version = "latest",
    [string]$BinDir = "$env:LOCALAPPDATA\bin"
)

function Write-Status {
    param([string]$Message, [string]$Type = "Info")
    $color = switch ($Type) {
        "Success" { "Green" }
        "Warning" { "Yellow" }
        "Error" { "Red" }
        default { "White" }
    }
    Write-Host $Message -ForegroundColor $color
}

function Test-CommandExists {
    param([string]$Command)
    $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

function Install-GitHubCLI {
    Write-Status "Installing GitHub CLI..." "Info"

    if (Test-CommandExists "winget") {
        try {
            winget install --id GitHub.cli --silent
            Write-Status "GitHub CLI installed successfully via winget" "Success"
            return $true
        }
        catch {
            Write-Status "Failed to install GitHub CLI via winget: $($_.Exception.Message)" "Warning"
        }
    }

    if (Test-CommandExists "choco") {
        try {
            choco install gh -y
            Write-Status "GitHub CLI installed successfully via chocolatey" "Success"
            return $true
        }
        catch {
            Write-Status "Failed to install GitHub CLI via chocolatey: $($_.Exception.Message)" "Warning"
        }
    }

    Write-Status "Please install GitHub CLI manually from https://cli.github.com/" "Warning"
    return $false
}

function Install-WindowsTerminal {
    Write-Status "Installing Windows Terminal..." "Info"

    if (Test-CommandExists "winget") {
        try {
            winget install --id Microsoft.WindowsTerminal --silent
            Write-Status "Windows Terminal installed successfully" "Success"
            return $true
        }
        catch {
            Write-Status "Failed to install Windows Terminal: $($_.Exception.Message)" "Warning"
        }
    }

    Write-Status "Please install Windows Terminal from Microsoft Store" "Warning"
    return $false
}

function Test-Dependencies {
    Write-Status "Checking dependencies..." "Info"

    $allDepsOk = $true

    # Check GitHub CLI
    if (-not (Test-CommandExists "gh")) {
        Write-Status "GitHub CLI not found. Installing..." "Warning"
        if (-not (Install-GitHubCLI)) {
            $allDepsOk = $false
        }
    } else {
        Write-Status "GitHub CLI is already installed" "Success"
    }

    # Check Windows Terminal (optional but recommended)
    if (-not (Test-CommandExists "wt")) {
        Write-Status "Windows Terminal not found. Installing..." "Warning"
        Install-WindowsTerminal  # Don't fail if this doesn't work
    } else {
        Write-Status "Windows Terminal is already installed" "Success"
    }

    return $allDepsOk
}

function Get-LatestVersion {
    try {
        $apiUrl = "https://api.github.com/repos/smtg-ai/claude-squad/releases"
        $releases = Invoke-RestMethod -Uri $apiUrl -Method Get

        if ($releases -and $releases.Count -gt 0) {
            $latestVersion = $releases[0].tag_name -replace "^v", ""
            return $latestVersion
        }
        else {
            throw "No releases found"
        }
    }
    catch {
        Write-Status "Failed to get latest version: $($_.Exception.Message)" "Error"
        throw
    }
}

function Get-Architecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { return "amd64" }
    }
}

function Download-Release {
    param(
        [string]$Version,
        [string]$Architecture,
        [string]$TempDir
    )

    $platform = "windows"
    $archiveExt = ".zip"

    $archiveName = "claude-squad_${Version}_${platform}_${Architecture}${archiveExt}"
    $downloadUrl = "https://github.com/smtg-ai/claude-squad/releases/download/v${Version}/${archiveName}"
    $downloadPath = Join-Path $TempDir $archiveName

    Write-Status "Downloading from: $downloadUrl" "Info"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $downloadPath -UseBasicParsing
        Write-Status "Download completed" "Success"
        return $downloadPath
    }
    catch {
        Write-Status "Download failed: $($_.Exception.Message)" "Error"
        throw
    }
}

function Extract-Archive {
    param(
        [string]$ArchivePath,
        [string]$ExtractDir
    )

    try {
        Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDir -Force
        Write-Status "Archive extracted successfully" "Success"
        return $true
    }
    catch {
        Write-Status "Failed to extract archive: $($_.Exception.Message)" "Error"
        return $false
    }
}

function Install-Binary {
    param(
        [string]$ExtractDir,
        [string]$BinDir,
        [string]$InstallName
    )

    $sourcePath = Join-Path $ExtractDir "claude-squad.exe"
    $targetPath = Join-Path $BinDir "$InstallName.exe"

    # Create bin directory if it doesn't exist
    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
        Write-Status "Created directory: $BinDir" "Info"
    }

    # Remove existing binary if upgrading
    if (Test-Path $targetPath) {
        Remove-Item $targetPath -Force
        Write-Status "Removed existing binary: $targetPath" "Info"
    }

    # Copy new binary
    try {
        Copy-Item $sourcePath $targetPath -Force
        Write-Status "Binary installed to: $targetPath" "Success"
        return $true
    }
    catch {
        Write-Status "Failed to install binary: $($_.Exception.Message)" "Error"
        return $false
    }
}

function Add-ToPath {
    param([string]$BinDir)

    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")

    if ($currentPath -notlike "*$BinDir*") {
        $newPath = "$currentPath;$BinDir"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Status "Added $BinDir to PATH" "Success"
        Write-Status "Please restart your terminal or run: `$env:PATH += ';$BinDir'" "Info"
    } else {
        Write-Status "$BinDir is already in PATH" "Info"
    }
}

function Test-Installation {
    param([string]$BinDir, [string]$InstallName)

    $binaryPath = Join-Path $BinDir "$InstallName.exe"

    if (Test-Path $binaryPath) {
        try {
            # Add to current session PATH for testing
            $env:PATH += ";$BinDir"
            $versionOutput = & $binaryPath version 2>&1
            Write-Status "Installation successful!" "Success"
            Write-Status "Version: $versionOutput" "Info"
            return $true
        }
        catch {
            Write-Status "Binary exists but failed to run: $($_.Exception.Message)" "Error"
            return $false
        }
    } else {
        Write-Status "Binary not found at: $binaryPath" "Error"
        return $false
    }
}

# Main installation process
function Main {
    Write-Status "Claude Squad Windows Installer" "Info"
    Write-Status "================================" "Info"

    # Check dependencies
    if (-not (Test-Dependencies)) {
        Write-Status "Dependency check failed. Please install missing dependencies and try again." "Error"
        exit 1
    }

    # Get version
    if ($Version -eq "latest") {
        try {
            $Version = Get-LatestVersion
            Write-Status "Latest version: $Version" "Info"
        }
        catch {
            Write-Status "Failed to get latest version. Please specify a version manually." "Error"
            exit 1
        }
    }

    # Detect architecture
    $architecture = Get-Architecture
    Write-Status "Detected architecture: $architecture" "Info"

    # Create temporary directory
    $tempDir = Join-Path $env:TEMP "claude-squad-install"
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        # Download release
        $archivePath = Download-Release -Version $Version -Architecture $architecture -TempDir $tempDir

        # Extract archive
        $extractDir = Join-Path $tempDir "extract"
        if (-not (Extract-Archive -ArchivePath $archivePath -ExtractDir $extractDir)) {
            exit 1
        }

        # Install binary
        if (-not (Install-Binary -ExtractDir $extractDir -BinDir $BinDir -InstallName $Name)) {
            exit 1
        }

        # Add to PATH
        Add-ToPath -BinDir $BinDir

        # Test installation
        if (Test-Installation -BinDir $BinDir -InstallName $Name) {
            Write-Status "" "Info"
            Write-Status "Installation completed successfully!" "Success"
            Write-Status "You can now use '$Name' command" "Info"
        } else {
            Write-Status "Installation verification failed" "Error"
            exit 1
        }
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# Run main function
Main
