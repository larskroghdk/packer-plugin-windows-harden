#Requires -Version 5.1

<#
.SYNOPSIS
    Reads and sets service start types.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1. Reads the start type from the registry so
    a missing service is reported as absent, and changes it with Set-Service.

.EXAMPLE
    . .\SecureWindows.Services.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>


function Get-SWServiceStartType {
    <#
    .SYNOPSIS
        Reads a service's start type from the registry.

    .DESCRIPTION
        Reads the Start value under SYSTEM\CurrentControlSet\Services. A service that is not
        installed is reported as absent, which the plugin treats as not applicable.

    .PARAMETER Root
        The open root key, normally from Get-SWRegistryRoot.

    .PARAMETER Name
        The service name (not the display name).

    .EXAMPLE
        Get-SWServiceStartType -Root $root -Name 'XblGameSave'

    .OUTPUTS
        System.Management.Automation.PSCustomObject with present, type and value.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [Microsoft.Win32.RegistryKey]$Root,
        [Parameter(Mandatory)]
        [ValidatePattern('^[A-Za-z0-9_.\-]+$')]
        [string]$Name
    )
    Get-SWRegistryValue -Root $Root -Path "SYSTEM\CurrentControlSet\Services\$Name" -Name 'Start'
}

function Set-SWServiceStartType {
    <#
    .SYNOPSIS
        Changes a service's start type.

    .DESCRIPTION
        Maps the baseline start type to Set-Service: 2 Automatic, 3 Manual, 4 Disabled. The
        service is not started or stopped.

    .PARAMETER Name
        The service name (not the display name).

    .PARAMETER StartType
        2, 3 or 4.

    .EXAMPLE
        Set-SWServiceStartType -Name 'XblGameSave' -StartType 4

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [ValidatePattern('^[A-Za-z0-9_.\-]+$')]
        [string]$Name,
        [Parameter(Mandatory)]
        [ValidateSet(2, 3, 4)]
        [int]$StartType
    )
    $startupType = @{ 2 = 'Automatic'; 3 = 'Manual'; 4 = 'Disabled' }[$StartType]
    Set-Service -Name $Name -StartupType $startupType -ErrorAction Stop
}
