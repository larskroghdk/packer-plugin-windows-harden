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
    Version : 1.0.0
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
}
