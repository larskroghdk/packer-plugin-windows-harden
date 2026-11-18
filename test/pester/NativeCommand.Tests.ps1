#Requires -Version 5.1

<#
.SYNOPSIS
    Pester tests for SecureWindows.NativeCommand.ps1.

.DESCRIPTION
    Checks that stderr output and non-zero exit codes are captured, not thrown, when the
    caller uses ErrorActionPreference Stop. Windows only.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\NativeCommand.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>

BeforeDiscovery {
    $script:notWindows = $env:OS -ne 'Windows_NT'
}

BeforeAll {
    . (Join-Path -Path $PSScriptRoot -ChildPath '..\..\guest\scripts\SecureWindows.NativeCommand.ps1')
    $script:cmd = Join-Path -Path $env:SystemRoot -ChildPath 'System32\cmd.exe'
}

Describe 'Invoke-SWNativeCommand' -Skip:$notWindows {
    It 'returns the exit code and output when the command writes to stderr under ErrorActionPreference Stop' {
        $ErrorActionPreference = 'Stop'
        $result = Invoke-SWNativeCommand -FilePath $cmd -ArgumentList @('/c', 'echo out& echo err 1>&2& exit 5')
        $result.ExitCode | Should -Be 5
        $result.Output | Should -Match 'out'
        $result.Output | Should -Match 'err'
    }
    It 'returns exit code 0 for a successful command' {
        (Invoke-SWNativeCommand -FilePath $cmd -ArgumentList @('/c', 'exit 0')).ExitCode | Should -Be 0
    }
    It 'leaves the caller error preference unchanged' {
        $ErrorActionPreference = 'Stop'

        $null = Invoke-SWNativeCommand -FilePath $cmd -ArgumentList @('/c', 'exit 0')

        $ErrorActionPreference | Should -Be 'Stop'
    }
}
