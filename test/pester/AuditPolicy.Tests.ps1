#Requires -Version 5.1

<#
.SYNOPSIS
    Pester tests for SecureWindows.AuditPolicy.ps1.

.DESCRIPTION
    Covers subcategory lookup in an exported audit policy. auditpol.exe itself is not
    called.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\AuditPolicy.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>

BeforeAll {
    . (Join-Path -Path $PSScriptRoot -ChildPath '..\..\guest\scripts\SecureWindows.AuditPolicy.ps1')
}

Describe 'Get-SWAuditSubcategory' {
    BeforeAll {
        $script:policy = @{ '0cce923f-69ae-11d9-bed3-505054503030' = 3 }
    }
    It 'matches GUIDs with braces and any case' {
        $state = Get-SWAuditSubcategory -Policy $policy -Guid '{0CCE923F-69AE-11D9-BED3-505054503030}'

        $state.present | Should -BeTrue
        $state.value | Should -BeExactly '3'
    }
    It 'reports an unknown subcategory as absent' {
        $state = Get-SWAuditSubcategory -Policy $policy -Guid '0cce9237-69ae-11d9-bed3-505054503030'

        $state.present | Should -BeFalse
        $state.value | Should -BeNullOrEmpty
    }
}
