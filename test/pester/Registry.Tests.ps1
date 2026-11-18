#Requires -Version 5.1

<#
.SYNOPSIS
    Pester tests for SecureWindows.Registry.ps1.

.DESCRIPTION
    Covers DWORD sign conversion and the read, write, remove and restore round trips
    against a temporary key under HKCU, which is deleted afterwards. The registry tests
    run on Windows only.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\Registry.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>

BeforeDiscovery {
    $script:notWindows = $env:OS -ne 'Windows_NT'
}

BeforeAll {
    . (Join-Path -Path $PSScriptRoot -ChildPath '..\..\guest\scripts\SecureWindows.Registry.ps1')
}

Describe 'ConvertTo-SWUInt32 and ConvertTo-SWInt32' {
    It 'maps -1 and 4294967295 onto each other' {
        ConvertTo-SWUInt32 -Value -1 | Should -Be 4294967295
        ConvertTo-SWInt32 -Value 4294967295 | Should -Be -1
    }
    It 'keeps small values unchanged' {
        ConvertTo-SWUInt32 -Value 5 | Should -Be 5
        ConvertTo-SWInt32 -Value 5 | Should -Be 5
    }
}

Describe 'Registry value functions' -Skip:$notWindows {
    BeforeAll {
        $script:root = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::CurrentUser, [Microsoft.Win32.RegistryView]::Registry64)
        $script:path = 'Software\SecureWindowsPester-' + [guid]::NewGuid().ToString('N')
    }
    AfterAll {
        $root.DeleteSubKeyTree($path, $false)
        $root.Dispose()
    }
    It 'reports a missing key as absent' {
        $state = Get-SWRegistryValue -Root $root -Path "$path\Missing" -Name 'Value'

        $state.present | Should -BeFalse
        $state.type | Should -BeNullOrEmpty
    }
    It 'round-trips a REG_DWORD above 2^31 as unsigned decimal' {
        Set-SWRegistryValue -Root $root -Path $path -Name 'Dword' -Type 'REG_DWORD' -Data '4294967295'
        $state = Get-SWRegistryValue -Root $root -Path $path -Name 'Dword'
        $state.present | Should -BeTrue
        $state.type | Should -Be 'REG_DWORD'
        $state.value | Should -BeExactly '4294967295'
    }
    It 'round-trips a REG_SZ' {
        Set-SWRegistryValue -Root $root -Path $path -Name 'String' -Type 'REG_SZ' -Data 'O:BAG:BAD:(A;;RC;;;BA)'
        $state = Get-SWRegistryValue -Root $root -Path $path -Name 'String'
        $state.type | Should -Be 'REG_SZ'
        $state.value | Should -BeExactly 'O:BAG:BAD:(A;;RC;;;BA)'
    }
    It 'reports a missing value in an existing key as absent' {
        (Get-SWRegistryValue -Root $root -Path $path -Name 'Nothing').present | Should -BeFalse
    }
    It 'removes a value and tolerates removing it again' {
        Set-SWRegistryValue -Root $root -Path $path -Name 'Gone' -Type 'REG_DWORD' -Data '1'
        Remove-SWRegistryValue -Root $root -Path $path -Name 'Gone'
        { Remove-SWRegistryValue -Root $root -Path $path -Name 'Gone' } | Should -Not -Throw
        { Remove-SWRegistryValue -Root $root -Path "$path\NoKey" -Name 'Gone' } | Should -Not -Throw
        (Get-SWRegistryValue -Root $root -Path $path -Name 'Gone').present | Should -BeFalse
    }
    It 'rejects a REG_DWORD that is not a decimal uint32' {
        { Set-SWRegistryValue -Root $root -Path $path -Name 'Bad' -Type 'REG_DWORD' -Data '-1' } | Should -Throw
    }
    It 'restores a previous value' {
        Set-SWRegistryValue -Root $root -Path $path -Name 'Restore' -Type 'REG_DWORD' -Data '5'
        Restore-SWRegistryValue -Root $root -Path $path -Name 'Restore' -State ([PSCustomObject]@{ present = $true; type = 'REG_DWORD'; value = '2' })

        (Get-SWRegistryValue -Root $root -Path $path -Name 'Restore').value | Should -Be '2'
    }
    It 'restores an absent value by deleting it' {
        Set-SWRegistryValue -Root $root -Path $path -Name 'WasAbsent' -Type 'REG_SZ' -Data 'x'
        Restore-SWRegistryValue -Root $root -Path $path -Name 'WasAbsent' -State ([PSCustomObject]@{ present = $false; type = $null; value = $null })

        (Get-SWRegistryValue -Root $root -Path $path -Name 'WasAbsent').present | Should -BeFalse
    }
    It 'refuses to restore an unsupported type' {
        $state = [PSCustomObject]@{ present = $true; type = 'REG_BINARY'; value = '1,2' }

        { Restore-SWRegistryValue -Root $root -Path $path -Name 'Binary' -State $state } | Should -Throw '*REG_BINARY*'
    }
}
