#Requires -Version 5.1

<#
.SYNOPSIS
    Manages the scheduled task that rolls back guarded settings.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1.
    Registers and removes the one-time SYSTEM task that runs the Rollback action if the plugin does not disarm it in time.

.EXAMPLE
    . .\SecureWindows.Guard.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.1.0
#>


function New-SWGuardClaim {
    <#
    .SYNOPSIS
        Atomically claims a guard snapshot for rollback or disarming.

    .DESCRIPTION
        Creates a sidecar file with CreateNew so that rollback and disarming cannot both win.
        The file remains until the plugin removes its working directory.

    .PARAMETER SnapshotPath
        The guard snapshot path shared by the scheduled rollback and disarm request.

    .EXAMPLE
        New-SWGuardClaim -SnapshotPath 'C:\Windows\Temp\sw\guard-snapshot.json'

    .OUTPUTS
        System.Boolean. True for the first caller, false for later callers.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([bool])]
    param (
        [Parameter(Mandatory)]
        [string]$SnapshotPath
    )
    $claimPath = "$SnapshotPath.claim"
    try {
        $claim = [IO.File]::Open($claimPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        $claim.Dispose()
        $true
    }
    catch [IO.IOException] {
        if (-not [IO.File]::Exists($claimPath)) { throw }
        $false
    }
}

function Register-SWGuardTask {
    <#
    .SYNOPSIS
        Schedules an automatic rollback of guarded registry changes.

    .DESCRIPTION
        Registers a one-time scheduled task that runs as SYSTEM after the delay.
        The task runs this script's Rollback action against the snapshot.
        Unless the plugin disarms it first, the guarded settings return to their previous values even if WinRM is no longer reachable.
        An existing task with the same name is replaced.

    .PARAMETER TaskName
        The task name in the root task folder. Letters, digits and hyphens only.

    .PARAMETER DelaySeconds
        Seconds from now until the rollback runs, between 30 and 3600.

    .PARAMETER ScriptPath
        Full path of Invoke-SecureWindows.ps1 on the guest.

    .PARAMETER SnapshotPath
        The snapshot JSON file uploaded by the plugin.

    .PARAMETER ResultPath
        Where the rollback writes its result.
        The plugin checks for this file to tell whether the rollback ran.

    .EXAMPLE
        Register-SWGuardTask -TaskName 'packer-windows-harden-guard-1' -DelaySeconds 180 -ScriptPath $script -SnapshotPath $snapshot -ResultPath $result

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [ValidatePattern('^[A-Za-z0-9\-]+$')]
        [string]$TaskName,
        [Parameter(Mandatory)]
        [ValidateRange(30, 3600)]
        [int]$DelaySeconds,
        [Parameter(Mandatory)]
        [string]$ScriptPath,
        [Parameter(Mandatory)]
        [string]$SnapshotPath,
        [Parameter(Mandatory)]
        [string]$ResultPath
    )
    $powershell = Join-Path -Path $env:SystemRoot -ChildPath 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $argument = '-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "{0}" -Action Rollback -InputPath "{1}" -OutputPath "{2}"' -f $ScriptPath, $SnapshotPath, $ResultPath
    $system = ([System.Security.Principal.SecurityIdentifier]'S-1-5-18').Translate([System.Security.Principal.NTAccount]).Value
    $task = @{
        TaskName  = $TaskName
        TaskPath  = '\'
        Action    = New-ScheduledTaskAction -Execute $powershell -Argument $argument
        Trigger   = New-ScheduledTaskTrigger -Once -At (Get-Date).AddSeconds($DelaySeconds)
        Principal = New-ScheduledTaskPrincipal -UserId $system -LogonType ServiceAccount -RunLevel Highest
        Settings  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit (New-TimeSpan -Minutes 5)
        Force     = $true
    }
    $null = Register-ScheduledTask @task -ErrorAction Stop
}

function Unregister-SWGuardTask {
    <#
    .SYNOPSIS
        Removes the rollback task if it still exists.

    .DESCRIPTION
        Returns $true when the task existed and was removed, and $false when it was already
        gone.

    .PARAMETER TaskName
        The task name in the root task folder.

    .EXAMPLE
        Unregister-SWGuardTask -TaskName 'packer-windows-harden-guard-1'

    .OUTPUTS
        System.Boolean

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([bool])]
    param (
        [Parameter(Mandatory)]
        [ValidatePattern('^[A-Za-z0-9\-]+$')]
        [string]$TaskName
    )
    $existing = Get-ScheduledTask -TaskName $TaskName -TaskPath '\' -ErrorAction SilentlyContinue
    if ($null -eq $existing) {
        return $false
    }
    Unregister-ScheduledTask -TaskName $TaskName -TaskPath '\' -Confirm:$false -ErrorAction Stop
    $true
}
