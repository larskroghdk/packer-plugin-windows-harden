#Requires -Version 5.1

<#
.SYNOPSIS
    Pester tests for SecureWindows.SecurityPolicy.ps1.

.DESCRIPTION
    Covers security template parsing, SID normalization and the user right and account
    policy lookups. secedit.exe itself is not called.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\SecurityPolicy.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>

BeforeAll {
    . (Join-Path -Path $PSScriptRoot -ChildPath '..\..\guest\scripts\SecureWindows.SecurityPolicy.ps1')
}

Describe 'ConvertFrom-SWSecurityTemplate' {
    It 'parses sections and entries, skipping blanks and comments' {
        $text = "[Unicode]`r`nUnicode=yes`r`n; note`r`n`r`n[System Access]`r`nMinimumPasswordLength = 14`r`n[Privilege Rights]`r`nSeNetworkLogonRight = *S-1-5-32-544,*S-1-5-11"
        $policy = ConvertFrom-SWSecurityTemplate -Text $text
        $policy['System Access']['MinimumPasswordLength'] | Should -Be '14'
        $policy['Privilege Rights']['SeNetworkLogonRight'] | Should -Be '*S-1-5-32-544,*S-1-5-11'
        $policy['Unicode'].Count | Should -Be 1
    }
    It 'looks up keys case-insensitively' {
        $policy = ConvertFrom-SWSecurityTemplate -Text "[System Access]`nPasswordComplexity=1"

        $policy['System Access']['passwordcomplexity'] | Should -Be '1'
    }
    It 'ignores entries before the first section and lines without a key' {
        $policy = ConvertFrom-SWSecurityTemplate -Text "Orphan=1`n[Section]`n=value`nKey=v"

        $policy.Keys | Should -Be @('Section')
        $policy['Section'].Keys | Should -Be @('Key')
    }
    It 'returns no sections for empty text' {
        (ConvertFrom-SWSecurityTemplate -Text '').Count | Should -Be 0
    }
}

Describe 'ConvertTo-SWSidList' {
    It 'strips the asterisk, upper-cases, sorts and de-duplicates SIDs' {
        ConvertTo-SWSidList -Value '*s-1-5-32-555, *S-1-5-32-544,*S-1-5-32-544' | Should -Be 'S-1-5-32-544,S-1-5-32-555'
    }
    It 'returns an empty string for no one' {
        ConvertTo-SWSidList -Value '' | Should -BeExactly ''
    }
    It 'keeps names that cannot be translated' -Skip:($env:OS -ne 'Windows_NT') {
        ConvertTo-SWSidList -Value 'NoSuchAccount-7f3c9a' | Should -Be 'NoSuchAccount-7f3c9a'
    }
    It 'translates account names to SIDs' -Skip:($env:OS -ne 'Windows_NT') {
        $everyone = ([System.Security.Principal.SecurityIdentifier]'S-1-1-0').Translate([System.Security.Principal.NTAccount]).Value
        ConvertTo-SWSidList -Value $everyone | Should -Be 'S-1-1-0'
    }
}

Describe 'Get-SWUserRight' {
    It 'reports an unassigned right as present with no one' {
        $state = Get-SWUserRight -Policy @{} -Right 'SeTcbPrivilege'
        $state.present | Should -BeTrue
        $state.value | Should -BeExactly ''
    }
    It 'normalises the assigned SIDs' {
        $policy = @{ 'Privilege Rights' = @{ 'SeBackupPrivilege' = '*S-1-5-32-551,*S-1-5-32-544' } }
        (Get-SWUserRight -Policy $policy -Right 'SeBackupPrivilege').value | Should -Be 'S-1-5-32-544,S-1-5-32-551'
    }
}

Describe 'Get-SWAccountPolicy' {
    It 'returns the configured value' {
        $policy = @{ 'System Access' = @{ 'LockoutBadCount' = '3' } }
        $state = Get-SWAccountPolicy -Policy $policy -Name 'LockoutBadCount'
        $state.present | Should -BeTrue
        $state.value | Should -Be '3'
    }
    It 'reports a missing entry as absent' {
        $state = Get-SWAccountPolicy -Policy @{ 'System Access' = @{} } -Name 'LockoutBadCount'
        $state.present | Should -BeFalse
        $state.value | Should -BeNullOrEmpty
    }
}
