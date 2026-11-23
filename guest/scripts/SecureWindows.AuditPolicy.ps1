#Requires -Version 5.1

<#
.SYNOPSIS
    Reads and writes advanced audit policy subcategories.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1. Wraps auditpol.exe /backup and /set.
    Requires Invoke-SWNativeCommand from SecureWindows.NativeCommand.ps1.

.EXAMPLE
    . .\SecureWindows.AuditPolicy.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>


function Get-SWAuditPolicy {
    <#
    .SYNOPSIS
        Exports the advanced audit policy subcategory settings.

    .DESCRIPTION
        Runs auditpol /backup and returns a hashtable that maps each lower-case subcategory GUID (without braces) to its setting: 0 none, 1 success, 2 failure, 3 both.
        The CSV file is deleted afterwards.

    .PARAMETER WorkPath
        A writable directory for the temporary CSV file.

    .EXAMPLE
        $auditPolicy = Get-SWAuditPolicy -WorkPath $workPath

    .OUTPUTS
        System.Collections.Hashtable

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([hashtable])]
    param(
        [Parameter(Mandatory)]
        [string]$WorkPath
    )
    $auditpol = Join-Path -Path $env:SystemRoot -ChildPath 'System32\auditpol.exe'
    $csv = Join-Path -Path $WorkPath -ChildPath 'auditpol-backup.csv'
    Remove-Item -LiteralPath $csv -Force -ErrorAction SilentlyContinue
    $result = Invoke-SWNativeCommand -FilePath $auditpol -ArgumentList @('/backup', "/file:$csv")
    if ($result.ExitCode -ne 0) {
        throw "auditpol.exe /backup failed with exit code $($result.ExitCode): $($result.Output)"
    }
    try {
        $header = 'Machine', 'Target', 'Subcategory', 'Guid', 'Inclusion', 'Exclusion', 'Value'
        $rows = Get-Content -LiteralPath $csv | Select-Object -Skip 1 | ConvertFrom-Csv -Header $header
        $policy = @{}
        foreach ($row in $rows) {
            if ([string]::IsNullOrWhiteSpace($row.Guid) -or [string]::IsNullOrWhiteSpace($row.Value)) {
                continue
            }
            $policy[$row.Guid.Trim('{', '}', ' ').ToLowerInvariant()] = [int]$row.Value
        }
        $policy
    }
    finally {
        Remove-Item -LiteralPath $csv -Force -ErrorAction SilentlyContinue
    }
}

function Get-SWAuditSubcategory {
    <#
    .SYNOPSIS
        Reads one audit subcategory setting from an exported audit policy.

    .DESCRIPTION
        Matches the GUID regardless of braces or case.
        A subcategory missing from the export is reported as absent.

    .PARAMETER Policy
        The policy returned by Get-SWAuditPolicy.

    .PARAMETER Guid
        The subcategory GUID.

    .EXAMPLE
        Get-SWAuditSubcategory -Policy $auditPolicy -Guid '0cce923f-69ae-11d9-bed3-505054503030'

    .OUTPUTS
        System.Management.Automation.PSCustomObject with present, type and value.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Policy,
        [Parameter(Mandatory)]
        [string]$Guid
    )
    $key = $Guid.Trim('{', '}', ' ').ToLowerInvariant()
    if ($Policy.ContainsKey($key)) {
        return [PSCustomObject]@{ present = $true; type = $null; value = [string]$Policy[$key] }
    }
    [PSCustomObject]@{ present = $false; type = $null; value = $null }
}

function Set-SWAuditSubcategory {
    <#
    .SYNOPSIS
        Sets success and failure auditing for one subcategory.

    .DESCRIPTION
        Runs auditpol /set.
        Bit 1 of the setting enables success auditing and bit 2 enables failure auditing.
        Throws with auditpol's output if it fails.

    .PARAMETER Guid
        The subcategory GUID.

    .PARAMETER Setting
        0 for no auditing, 1 for success, 2 for failure, 3 for both.

    .EXAMPLE
        Set-SWAuditSubcategory -Guid '0cce923f-69ae-11d9-bed3-505054503030' -Setting 3

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory)]
        [string]$Guid,
        [Parameter(Mandatory)]
        [ValidateRange(0, 3)]
        [int]$Setting
    )
    $auditpol = Join-Path -Path $env:SystemRoot -ChildPath 'System32\auditpol.exe'
    $success = if ($Setting -band 1) { 'enable' } else { 'disable' }
    $failure = if ($Setting -band 2) { 'enable' } else { 'disable' }
    $subcategory = '/subcategory:{{{0}}}' -f $Guid.Trim('{', '}')
    $result = Invoke-SWNativeCommand -FilePath $auditpol -ArgumentList @('/set', $subcategory, "/success:$success", "/failure:$failure")
    if ($result.ExitCode -ne 0) {
        throw "auditpol.exe /set $subcategory failed with exit code $($result.ExitCode): $($result.Output)"
    }

}
