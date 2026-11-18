#Requires -Version 5.1

<#
.SYNOPSIS
    Runs one windows-harden guest action and writes its result as JSON.

.DESCRIPTION
    Entry point that the Packer plugin starts over WinRM, once per action. It loads the
    SecureWindows.*.ps1 parts from its own directory, reads the JSON request from
    -InputPath when the action takes one, and writes the JSON result to -OutputPath.
    The script only detects, reads and writes state. Every decision is made by the plugin.
    Detect      collects operating system, session, WinRM and firewall facts.
    Probe       re-reads WinRM, the session and the boot time to check connectivity.
    Inspect     reads the current state of the requested settings.
    Apply       writes the requested settings.
    ArmGuard    schedules the rollback task for guarded settings.
    DisarmGuard removes that task and reports whether it already ran.
    Rollback    restores registry values from a snapshot.
    Exits 0 when the action ran, even if individual settings failed. Exits 1 when the
    action itself failed, with the error on stderr.

.PARAMETER Action
    The action to run.

.PARAMETER InputPath
    JSON request file. Required by every action except Detect and Probe.

.PARAMETER OutputPath
    Where the JSON result is written. Its directory is also used for temporary files.

.EXAMPLE
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Invoke-SecureWindows.ps1 -Action Detect -OutputPath C:\Windows\Temp\sw\01-detect-output.json

.EXAMPLE
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Invoke-SecureWindows.ps1 -Action Inspect -InputPath C:\Windows\Temp\sw\04-inspect-input.json -OutputPath C:\Windows\Temp\sw\04-inspect-output.json

.OUTPUTS
    None. The result is written to -OutputPath. Progress lines go to stdout.

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>

[CmdletBinding()]
param (
    [Parameter(Mandatory)]
    [ValidateSet(
        'Detect',
        'Probe',
        'Inspect',
        'Apply',
        'ArmGuard',
        'DisarmGuard',
        'Rollback'
    )]
    [string]$Action,
    [Parameter()]
    [string]$InputPath,
    [Parameter(Mandatory)]
    [string]$OutputPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

trap {
    [Console]::Error.WriteLine("ERROR: $($_.Exception.Message)")
    [Console]::Error.WriteLine("ERROR: $($_.ScriptStackTrace)")
    exit 1
}

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
foreach ($part in 'NativeCommand', 'Registry', 'SecurityPolicy', 'AuditPolicy', 'Services', 'Facts', 'Guard') {
    . (Join-Path -Path $scriptRoot -ChildPath "SecureWindows.$part.ps1")
}

$workPath = Split-Path -Parent $OutputPath

function Write-SWLog {
    <#
    .SYNOPSIS
        Writes a progress line to standard output.

    .DESCRIPTION
        Writes the message straight to the console output stream so that Packer shows it in
        the build log. It bypasses the PowerShell pipeline, so the line never ends up in a
        function's return value.

    .PARAMETER Message
        The line to write. Must not contain secrets, because Packer logs it.

    .EXAMPLE
        Write-SWLog "Set service XblGameSave start type = 4"

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Message
    )
    [Console]::Out.WriteLine($Message)
}

function Read-SWInput {
    <#
    .SYNOPSIS
        Reads the JSON request that the plugin uploaded for the current action.

    .DESCRIPTION
        Parses the file named by the script's -InputPath parameter as UTF-8 JSON. A JSON
        array is written to the pipeline one element at a time, so wrapping the call in @()
        gives the same result in Windows PowerShell 5.1 and PowerShell 7.

    .EXAMPLE
        $settings = @(Read-SWInput)

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    if ([string]::IsNullOrEmpty($InputPath)) {
        throw "Action $Action requires -InputPath"
    }
    $parsed = [IO.File]::ReadAllText($InputPath, [Text.Encoding]::UTF8) | ConvertFrom-Json
    $parsed
}

function Write-SWResult {
    <#
    .SYNOPSIS
        Writes the action result to the output file as JSON.

    .DESCRIPTION
        Serializes the result to compact JSON and writes it to the script's -OutputPath as
        UTF-8 without a byte order mark. The plugin downloads and decodes this file.

    .PARAMETER Result
        The object to serialize. Property names are the JSON field names the Go side decodes.

    .EXAMPLE
        Write-SWResult -Result ([PSCustomObject]@{ armed = $true })

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory)]
        [object]$Result
    )
    $json = ConvertTo-Json -InputObject $Result -Depth 10 -Compress
    [IO.File]::WriteAllText($OutputPath, $json, (New-Object System.Text.UTF8Encoding ($false)))
}

function Invoke-SWInspect {
    <#
    .SYNOPSIS
        Reads the current state of each baseline setting.

    .DESCRIPTION
        Returns one state object per setting, in input order. Security policy and audit policy
        are exported once and shared by every setting of those kinds. When an export fails,
        each affected setting reports that failure in its error field. A failure on one
        setting never stops the others.

    .PARAMETER Settings
        Baseline settings as sent by the plugin, each with an id, a kind and the payload for
        that kind.

    .EXAMPLE
        Invoke-SWInspect -Settings @(Read-SWInput)

    .OUTPUTS
        System.Management.Automation.PSCustomObject with id, present, type, value and error.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory)]
        [object[]]$Settings
    )
    $root = Get-SWRegistryRoot
    $kinds = @($Settings | ForEach-Object { $_.kind } | Sort-Object -Unique)
    $securityPolicy = $null
    $auditPolicy = $null
    $exportFailure = @{}
    if ($kinds -contains 'user_right' -or $kinds -contains 'account_policy') {
        try {
            $securityPolicy = Export-SWSecurityPolicy -WorkPath $workPath
        }
        catch {
            $exportFailure['user_right'] = $_.Exception.Message
            $exportFailure['account_policy'] = $_.Exception.Message
        }
    }
    if ($kinds -contains 'audit_policy') {
        try {
            $auditPolicy = Get-SWAuditPolicy -WorkPath $workPath
        }
        catch {
            $exportFailure['audit_policy'] = $_.Exception.Message
        }
    }
    try {
        foreach ($setting in $Settings) {
            $state = $null
            $failure = $null
            try {
                if ($exportFailure.ContainsKey($setting.kind)) {
                    throw $exportFailure[$setting.kind]
                }
                switch ($setting.kind) {
                    'registry' {
                        $state = Get-SWRegistryValue -Root $root -Path $setting.registry.path -Name $setting.registry.name
                    }
                    'user_right' {
                        $state = Get-SWUserRight -Policy $securityPolicy -Right $setting.user_right.right
                    }
                    'account_policy' {
                        $state = Get-SWAccountPolicy -Policy $securityPolicy -Name $setting.account_policy.name
                    }
                    'audit_policy' {
                        $state = Get-SWAuditSubcategory -Policy $auditPolicy -Guid $setting.audit_policy.subcategory_guid
                    }
                    'service' {
                        $state = Get-SWServiceStartType -Root $root -Name $setting.service.name
                    }
                    default {
                        throw "Unsupported setting kind '$($setting.kind)'"
                    }
                }
            }
            catch {
                $failure = $_.Exception.Message
            }
            [PSCustomObject]@{
                id      = $setting.id
                present = if ($null -ne $state) { $state.present } else { $false }
                type    = if ($null -ne $state) { $state.type } else { $null }
                value   = if ($null -ne $state) { $state.value } else { $null }
                error   = $failure
            }
        }
    }
    finally {
        $root.Dispose()
    }
}

function Invoke-SWApply {
    <#
    .SYNOPSIS
        Applies baseline settings to the machine.

    .DESCRIPTION
        Writes registry values, audit subcategories and service start types one at a time.
        User rights and account policies are collected and applied in a single secedit.exe
        run, so they all share that run's result. Each setting gets one result object, and a
        failure on one setting does not stop the rest.

    .PARAMETER Settings
        The settings to apply, already cleared by the plugin's WinRM protection rules.

    .EXAMPLE
        Invoke-SWApply -Settings @(Read-SWInput)

    .OUTPUTS
        System.Management.Automation.PSCustomObject with id, ok, error and warning.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [object[]]$Settings
    )
    $root = Get-SWRegistryRoot
    $results = New-Object System.Collections.Generic.List[object]
    $userRights = @{}
    $accountPolicies = @{}
    $seceditIds = New-Object System.Collections.Generic.List[string]
    try {
        foreach ($setting in $Settings) {
            $failure = $null
            $batched = $false
            try {
                switch ($setting.kind) {
                    'registry' {
                        $registry = $setting.registry
                        if ($registry.type -eq 'ABSENT') {
                            Remove-SWRegistryValue -Root $root -Path $registry.path -Name $registry.name
                            Write-SWLog "Removed HKLM\$($registry.path)\$($registry.name)"
                        }
                        else {
                            Set-SWRegistryValue -Root $root -Path $registry.path -Name $registry.name -Type $registry.type -Data $registry.data
                            Write-SWLog "Set HKLM\$($registry.path)\$($registry.name) = $($registry.data)"
                        }
                    }
                    'user_right' {
                        $userRights[$setting.user_right.right] = @($setting.user_right.sids)
                        $seceditIds.Add($setting.id)
                        $batched = $true
                    }
                    'account_policy' {
                        $accountPolicies[$setting.account_policy.name] = $setting.account_policy.value
                        $seceditIds.Add($setting.id)
                        $batched = $true
                    }
                    'audit_policy' {
                        Set-SWAuditSubcategory -Guid $setting.audit_policy.subcategory_guid -Setting $setting.audit_policy.setting
                        Write-SWLog "Set audit subcategory $($setting.audit_policy.subcategory) = $($setting.audit_policy.setting)"
                    }
                    'service' {
                        Set-SWServiceStartType -Name $setting.service.name -StartType $setting.service.start_type
                        Write-SWLog "Set service $($setting.service.name) start type = $($setting.service.start_type)"
                    }
                    default {
                        throw "Unsupported setting kind '$($setting.kind)'"
                    }
                }
            }
            catch {
                $failure = $_.Exception.Message
            }
            if (-not $batched) {
                $results.Add([PSCustomObject]@{ id = $setting.id; ok = ($null -eq $failure); error = $failure; warning = $null })
            }
        }
    }
    finally {
        $root.Dispose()
    }
    if ($seceditIds.Count -gt 0) {
        $failure = $null
        $warning = $null
        try {
            $code = Set-SWSecurityPolicy -WorkPath $workPath -UserRights $userRights -AccountPolicies $accountPolicies
            Write-SWLog "Applied $($seceditIds.Count) security policy entries with secedit.exe (exit code $code)"
            if ($code -eq 3) {
                $warning = 'secedit.exe completed with warnings (exit code 3); the result is verified separately'
            }
        }
        catch {
            $failure = $_.Exception.Message
        }
        foreach ($id in $seceditIds) {
            $results.Add([PSCustomObject]@{ id = $id; ok = ($null -eq $failure); error = $failure; warning = $warning })
        }
    }
    $results
}

function Invoke-SWRollback {
    <#
    .SYNOPSIS
        Restores registry values from a guard snapshot.

    .DESCRIPTION
        Puts each snapshotted registry value back to its recorded state, deleting values that
        were absent before. Only registry settings can be rolled back; any other kind is
        reported as failed.

    .PARAMETER Items
        Snapshot entries, each holding the setting and its state before the guarded change.

    .EXAMPLE
        Invoke-SWRollback -Items @($snapshot.items)

    .OUTPUTS
        System.Management.Automation.PSCustomObject with id, ok, error and warning.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory)]
        [object[]]$Items
    )
    $root = Get-SWRegistryRoot
    try {
        foreach ($item in $Items) {
            $failure = $null
            try {
                if ($item.setting.kind -ne 'registry') {
                    throw "Rollback supports registry settings only, got '$($item.setting.kind)'"
                }
                $registry = $item.setting.registry
                Restore-SWRegistryValue -Root $root -Path $registry.path -Name $registry.name -State $item.state
            }
            catch {
                $failure = $_.Exception.Message
            }
            [PSCustomObject]@{ id = $item.setting.id; ok = ($null -eq $failure); error = $failure; warning = $null }
        }
    }
    finally {
        $root.Dispose()
    }
}

switch ($Action) {
    'Detect' {
        Write-SWResult -Result (Get-SWSystemFacts)
    }
    'Probe' {
        $winrm = $null
        $failure = $null
        try {
            $winrm = Get-SWWinRMConfiguration
        }
        catch {
            $failure = $_.Exception.Message
        }
        $os = Get-CimInstance
        -ClassName Win32_OperatingSystem -ErrorAction Stop
        Write-SWResult -Result ([PSCustomObject]@{
                winrm     = $winrm
                session   = Get-SWSessionIdentity
                boot_time = $os.LastBootUpTime.ToUniversalTime().ToString('o', [Globalization.CultureInfo]::InvariantCulture)
                error     = $failure
            })
    }
    'Inspect' {
        $settings = @(Read-SWInput)
        Write-SWResult -Result ([PSCustomObject]@{ items = @(Invoke-SWInspect -Settings $settings) })
    }
    'Apply' {
        $settings = @(Read-SWInput)
        Write-SWResult -Result ([PSCustomObject]@{ items = @(Invoke-SWApply -Settings $settings) })
    }
    'ArmGuard' {
        $request = Read-SWInput
        $params = @{
            TaskName     = $request.task_name
            DelaySeconds = $request.delay_seconds
            ScriptPath   = $MyInvocation.MyCommand.Path
            SnapshotPath = $request.snapshot_path
            ResultPath   = $request.result_path
        }
        Register-SWGuardTask @params
        Write-SWResult -Result ([PSCustomObject]@{ armed = $true })
    }
    'DisarmGuard' {
        $request = Read-SWInput
        $existed = Unregister-SWGuardTask -TaskName $request.task_name
        $rolledBack = Test-Path -LiteralPath $request.result_path -PathType Leaf
        Write-SWResult -Result ([PSCustomObject]@{ existed = $existed; rolled_back = $rolledBack })
    }
    'Rollback' {
        $snapshot = Read-SWInput
        Write-SWResult -Result ([PSCustomObject]@{ items = @(Invoke-SWRollback -Items @($snapshot.items)) })
    }
}
exit 0
