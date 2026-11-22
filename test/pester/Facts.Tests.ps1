#Requires -Version 5.1

<#
.SYNOPSIS
    Tests the collection shapes used in the guest Detect JSON response.

.DESCRIPTION
    Mocks read-only collectors and checks empty, singleton and multiple role and connection
    results under Windows PowerShell 5.1. No machine configuration is changed.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\Facts.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>


BeforeAll {
    . (Join-Path $PSScriptRoot '..\..\guest\scripts\SecureWindows.Facts.ps1')
}

Describe 'Get-SWSystemFacts JSON arrays' {
    BeforeEach {
        Mock Get-SWOperatingSystem { [PSCustomObject]@{ product_type = 3 } }
        Mock Get-SWWinRMConfiguration { [PSCustomObject]@{ listeners = @([PSCustomObject]@{ port = 5985 }) } }
        Mock Get-SWSessionIdentity { [PSCustomObject]@{ is_elevated = $true } }
        Mock Get-SWDomainMembership { [PSCustomObject]@{ part_of_domain = $false } }
        Mock Get-SWFirewallState { $null }
        Mock Get-SWWinRMConnection {
            for ($i = 0; $i -lt $script:itemCount; $i++) { [PSCustomObject]@{ local_port = 5985 } }
        }
        Mock Get-SWInstalledRole {
            for ($i = 0; $i -lt $script:itemCount; $i++) { "Role$i" }
        }
    }
    It 'keeps <Count> collected connections and roles as arrays' -ForEach @(
        @{ Count = 0 }, @{ Count = 1 }, @{ Count = 2 }
    ) {
        $script:itemCount = $Count
        $facts = Get-SWSystemFacts
        ($facts.winrm_connections -is [array]) | Should -BeTrue
        ($facts.roles -is [array]) | Should -BeTrue
        $facts.winrm_connections.Count | Should -Be $Count
        $facts.roles.Count | Should -Be $Count
        $json = ConvertTo-Json -InputObject $facts -Depth 10 -Compress
        $json | Should -Match '"winrm_connections":\['
        $json | Should -Match '"roles":\['
    }
    It 'keeps failed collectors empty and records their errors' {
        Mock Get-SWWinRMConnection { throw 'connection collection failed' }
        Mock Get-SWInstalledRole { throw 'role collection failed' }
        $facts = Get-SWSystemFacts
        $facts.winrm_connections.Count | Should -Be 0
        $facts.roles.Count | Should -Be 0
        $facts.errors.Count | Should -Be 2
    }
}
