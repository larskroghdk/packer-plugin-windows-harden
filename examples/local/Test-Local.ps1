#Requires -Version 5.1

<#
.SYNOPSIS
    Builds and tests the local Windows hardening Packer plugin.

.DESCRIPTION
    Builds the repository plugin, installs it in an isolated directory under dist, validates the local template and runs it against an existing WinRM target.
    Runs in dry-run mode unless Apply is specified, even if the variable file sets dry_run to false.
    Requires Go and Packer on PATH and an existing variable file.

.PARAMETER VarFile
    Path to a Packer variable file.
    Defaults to local.pkrvars.hcl beside this script.
    Relative paths are resolved from the caller's working directory.

.PARAMETER Apply
    Applies hardening and permits a restart of the target.
    Use a disposable VM or take a checkpoint first.
    Cannot be combined with ValidateOnly.

.PARAMETER ValidateOnly
    Builds and validates without connecting to a Windows target.

.EXAMPLE
    PS> .\Test-Local.ps1
    Builds the plugin and runs a dry-run inspection using local.pkrvars.hcl.

.EXAMPLE
    PS> .\Test-Local.ps1 -VarFile .\local.pkrvars.hcl -ValidateOnly
    Checks the plugin and template without connecting to the example address.

.EXAMPLE
    PS> .\Test-Local.ps1 -Apply
    Applies and verifies hardening on the configured target, restarting if needed.

.NOTES
    Author: Lars Krogh
    Version: 1.0.0
#>

[CmdletBinding()]
param(
    [string]$VarFile = (Join-Path $PSScriptRoot 'local.pkrvars.hcl'),
    [switch]$Apply,
    [switch]$ValidateOnly
)

Set-StrictMode -Version Latest

$ErrorActionPreference = 'Stop'

if ($Apply -and $ValidateOnly) {
    throw 'Apply and ValidateOnly cannot be combined.'
}
foreach ($command in 'go', 'packer') {
    if (-not (Get-Command $command -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Required command '$command' was not found on PATH."
    }
}
if (-not (Test-Path -LiteralPath $VarFile -PathType Leaf)) {
    throw "Variable file '$VarFile' does not exist. Copy examples/local/local.pkrvars.hcl.example to local.pkrvars.hcl and configure the target and credentials."
}
$resolvedVarFile = (Resolve-Path -LiteralPath $VarFile).ProviderPath
if ([System.IO.Path]::GetExtension($resolvedVarFile) -notin '.hcl', '.json') {
    throw 'Packer variable files must end in .hcl or .json. Copy the example to local.pkrvars.hcl first.'
}

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).ProviderPath
$buildDirectory = Join-Path $repositoryRoot 'dist/local-test'
$pluginDirectory = Join-Path $buildDirectory 'plugins'
$binaryName = 'packer-plugin-windows-harden'
if ($env:OS -eq 'Windows_NT') {
    $binaryName += '.exe'
}
$binaryPath = Join-Path $buildDirectory $binaryName
$previousPluginPath = $env:PACKER_PLUGIN_PATH
$dryRunArgument = '-var=dry_run=true'
if ($Apply) {
    $dryRunArgument = '-var=dry_run=false'
}

try {
    New-Item -ItemType Directory -Path $pluginDirectory -Force | Out-Null
    $env:PACKER_PLUGIN_PATH = $pluginDirectory
    Push-Location $repositoryRoot
    try {
        & go build -o $binaryPath .
        if ($LASTEXITCODE -ne 0) {
            throw "Plugin build failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Pop-Location
    }
    & packer plugins install -force -path $binaryPath github.com/larskroghdk/windows-harden
    if ($LASTEXITCODE -ne 0) {
        throw "Plugin installation failed with exit code $LASTEXITCODE."
    }
    Push-Location $PSScriptRoot
    try {
        & packer validate "-var-file=$resolvedVarFile" $dryRunArgument .
        if ($LASTEXITCODE -ne 0) {
            throw "Template validation failed with exit code $LASTEXITCODE."
        }
        if ($ValidateOnly) {
            Write-Host 'Plugin build and template validation passed. No target connection was made.'
        }
        else {
            & packer build "-var-file=$resolvedVarFile" $dryRunArgument .
            if ($LASTEXITCODE -ne 0) {
                throw "Local Packer test failed with exit code $LASTEXITCODE. Inspect examples/local/reports/local.json if present."
            }
            $reportPath = Join-Path $PSScriptRoot 'reports/local.json'
            if (-not (Test-Path -LiteralPath $reportPath -PathType Leaf)) {
                throw "Packer finished without producing the expected report: $reportPath"
            }
            $report = Get-Content -LiteralPath $reportPath -Raw | ConvertFrom-Json
            if ($report.summary.failed -gt 0) {
                throw "The report contains $($report.summary.failed) failed settings. Inspect $reportPath"
            }
            Write-Host "Local Packer test passed. Report: $reportPath"
            $report.summary | Format-List
        }
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:PACKER_PLUGIN_PATH = $previousPluginPath
}
