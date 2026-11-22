#Requires -Version 5.1

<#
.SYNOPSIS
    Pester tests for the Invoke-SecureWindows.ps1 entry point.

.DESCRIPTION
    Runs the entry script in a child process, as the plugin does, and checks the JSON
    contract, the exit codes and the UTF-8 encoding without a byte order mark. Windows
    only.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\Invoke-SecureWindows.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.1.0
#>

BeforeDiscovery {
    $script:notWindows = $env:OS -ne 'Windows_NT'
}

Describe 'Invoke-SecureWindows.ps1' -Skip:$notWindows {
    BeforeAll {
        $entry = Join-Path -Path $PSScriptRoot -ChildPath '..\..\guest\scripts\Invoke-SecureWindows.ps1'
        $shell = (Get-Process -Id $PID).Path
        function Invoke-Entry {
            <#
            .SYNOPSIS
                Runs Invoke-SecureWindows.ps1 in a child process, as the plugin does.

            .DESCRIPTION
                Writes the input object as JSON to the test drive and runs the entry script with
                -File in the same PowerShell edition as the test. Returns the exit code and the path
                of the output file.

            .PARAMETER Action
                The guest action to run.

            .PARAMETER InputObject
                The request to serialize as the action input. When omitted, no -InputPath is passed.

            .EXAMPLE
                Invoke-Entry -Action 'Inspect' -InputObject $settings

            .OUTPUTS
                System.Management.Automation.PSCustomObject with ExitCode and OutputPath.

            .NOTES
                Author  : Lars Krogh
                Version : 1.0.0
            #>
            param (
                [string]$Action,
                [object]$InputObject
            )
            $outputPath = Join-Path -Path $TestDrive -ChildPath "$Action-output.json"
            $arguments = @('-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', $entry, '-Action', $Action, '-OutputPath', $outputPath)
            if ($null -ne $InputObject) {
                $inputPath = Join-Path -Path $TestDrive -ChildPath "$Action-input.json"
                [IO.File]::WriteAllText($inputPath, (ConvertTo-Json -InputObject $InputObject -Depth 10), (New-Object System.Text.UTF8Encoding ($false)))
                $arguments += @('-InputPath', $inputPath)
            }
            $null = & $shell @arguments 2>&1
            [PSCustomObject]@{
                ExitCode   = $LASTEXITCODE
                OutputPath = $outputPath
            }
        }
    }
    Context 'Inspect' {
        BeforeAll {
            $settings = @(
                [PSCustomObject]@{
                    id       = 'registry:HKLM\SOFTWARE\SecureWindowsPester\Missing'
                    kind     = 'registry'
                    registry = [PSCustomObject]@{ path = 'SOFTWARE\SecureWindowsPester'; name = 'Missing'; type = 'REG_DWORD'; data = '1' }
                },
                [PSCustomObject]@{
                    id      = 'service:SecureWindowsPesterNoSuchService'
                    kind    = 'service'
                    service = [PSCustomObject]@{ name = 'SecureWindowsPesterNoSuchService'; start_type = 4 }
                },
                [PSCustomObject]@{
                    id   = 'bogus:1'
                    kind = 'bogus'
                }
            )
            $run = Invoke-Entry -Action 'Inspect' -InputObject $settings
            $bytes = [IO.File]::ReadAllBytes($run.OutputPath)
            $script:result = [Text.Encoding]::UTF8.GetString($bytes) | ConvertFrom-Json
        }
        It 'exits 0 even when individual settings fail' {
            $run.ExitCode | Should -Be 0
        }
        It 'writes UTF-8 without a byte order mark' {
            $bytes[0] | Should -Not -Be 0xEF
        }
        It 'returns one item per setting with the fields the plugin decodes' {
            @($result.items).Count | Should -Be 3
            foreach ($item in $result.items) {
                $item.PSObject.Properties.Name | Should -Be @('id', 'present', 'type', 'value', 'error')
            }
        }
        It 'reports absent registry values and missing services as not present' {
            $result.items[0].present | Should -BeFalse
            $result.items[0].error | Should -BeNullOrEmpty
            $result.items[1].present | Should -BeFalse
            $result.items[1].error | Should -BeNullOrEmpty
        }

        It 'reports an unknown kind as a per-setting error' {
            $result.items[2].error | Should -BeLike "*Unsupported setting kind 'bogus'*"
        }
    }
    It 'exits 1 when an action that needs input gets none' {
        (Invoke-Entry -Action 'Inspect').ExitCode | Should -Be 1
    }
    Context 'Probe and guard coordination' {
        BeforeAll {
            $fixture = Join-Path $TestDrive 'guest'
            $null = New-Item -ItemType Directory -Path $fixture
            Copy-Item (Join-Path (Split-Path $entry) '*.ps1') $fixture
            $entry = Join-Path $fixture 'Invoke-SecureWindows.ps1'
            @'
function Get-SWWinRMConfiguration { [PSCustomObject]@{ service_status = 'Running' } }
function Get-SWSessionIdentity { [PSCustomObject]@{ is_elevated = $true } }
function Get-CimInstance {
    param([Parameter(Mandatory)][string]$ClassName)
    if ($ClassName -ne 'Win32_OperatingSystem') { throw 'Unexpected CIM class' }
    [PSCustomObject]@{ LastBootUpTime = [datetime]'2026-11-22T00:00:00Z' }
}
'@ | Set-Content -LiteralPath (Join-Path $fixture 'SecureWindows.Facts.ps1')
            @'
function Unregister-SWGuardTask { param([string]$TaskName) $true }
'@ | Add-Content -LiteralPath (Join-Path $fixture 'SecureWindows.Guard.ps1')
            . (Join-Path $fixture 'SecureWindows.Guard.ps1')
        }
        It 'returns a boot time and elevation from a noninteractive Probe command' {
            $run = Invoke-Entry -Action 'Probe'
            $run.ExitCode | Should -Be 0
            $probe = Get-Content -LiteralPath $run.OutputPath -Raw | ConvertFrom-Json
            $probe.boot_time | Should -BeLike '2026-11-22T*'
            $probe.session.is_elevated | Should -BeTrue
        }
        It 'prevents a rollback from running after disarming has claimed its snapshot' {
            $snapshotPath = Join-Path $TestDrive 'Rollback-input.json'
            New-SWGuardClaim -SnapshotPath $snapshotPath | Should -BeTrue
            $run = Invoke-Entry -Action 'Rollback' -InputObject ([PSCustomObject]@{ items = @() })
            $run.ExitCode | Should -Be 0
            Test-Path -LiteralPath $run.OutputPath | Should -BeFalse
        }
        It 'fails disarming while a rollback owns the snapshot but has not completed' {
            $snapshotPath = Join-Path $TestDrive 'running-snapshot.json'
            New-SWGuardClaim -SnapshotPath $snapshotPath | Should -BeTrue
            $request = [PSCustomObject]@{
                task_name = 'test-guard'
                snapshot_path = $snapshotPath
                result_path = Join-Path $TestDrive 'not-finished.json'
            }
            (Invoke-Entry -Action 'DisarmGuard' -InputObject $request).ExitCode | Should -Be 1
        }
    }
}
