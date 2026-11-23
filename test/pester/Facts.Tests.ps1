#Requires -Version 5.1

<#
.SYNOPSIS
    Tests the collection shapes used in the guest Detect JSON response.

.DESCRIPTION
    Mocks read-only collectors and checks empty, singleton and multiple role and connection results under Windows PowerShell 5.1.
    Also checks the local password status section and reads the password status of a real local account.
    No machine configuration is changed.

.EXAMPLE
    Invoke-Pester -Path .\test\pester\Facts.Tests.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.1.0
#>


BeforeAll {
    . (Join-Path $PSScriptRoot '..\..\guest\scripts\SecureWindows.Facts.ps1')
}

Describe 'Get-SWSystemFacts JSON arrays' {
    BeforeEach {
        Mock Get-SWOperatingSystem { [PSCustomObject]@{ product_type = 3 } }
        Mock Get-SWWinRMConfiguration { [PSCustomObject]@{ listeners = @([PSCustomObject]@{ port = 5985 }) } }
        Mock Get-SWSessionIdentity { [PSCustomObject]@{ is_elevated = $true; is_local_account = $false; user_sid = 'S-1-5-21-1-2-3-1001' } }
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

Describe 'Get-SWSystemFacts session password status' {
    BeforeEach {
        Mock Get-SWOperatingSystem { [PSCustomObject]@{ product_type = 1 } }
        Mock Get-SWWinRMConfiguration { throw 'not collected in this test' }
        Mock Get-SWDomainMembership { $null }
        Mock Get-SWFirewallState { $null }
        Mock Get-SWInstalledRole { $null }
        Mock Get-SWLocalPasswordStatus { [PSCustomObject]@{ age_seconds = 86400; never_expires = $false } }
    }
    It 'collects the status for a local account' {
        Mock Get-SWSessionIdentity { [PSCustomObject]@{ is_local_account = $true; user_sid = 'S-1-5-21-1-2-3-500' } }
        $facts = Get-SWSystemFacts
        $facts.session_password.age_seconds | Should -Be 86400
        Should -Invoke Get-SWLocalPasswordStatus -Times 1 -ParameterFilter { $UserSid -eq 'S-1-5-21-1-2-3-500' }
    }
    It 'leaves the status null for a domain account' {
        Mock Get-SWSessionIdentity { [PSCustomObject]@{ is_local_account = $false; user_sid = 'S-1-5-21-9-9-9-1105' } }
        $facts = Get-SWSystemFacts
        $facts.session_password | Should -BeNullOrEmpty
        Should -Invoke Get-SWLocalPasswordStatus -Times 0
        (ConvertTo-Json -InputObject $facts -Depth 10 -Compress) | Should -Match '"session_password":null'
    }
    It 'records an error and leaves the status null when collection fails' {
        Mock Get-SWSessionIdentity { [PSCustomObject]@{ is_local_account = $true; user_sid = 'S-1-5-21-1-2-3-500' } }
        Mock Get-SWLocalPasswordStatus { throw 'access denied' }
        $facts = Get-SWSystemFacts
        $facts.session_password | Should -BeNullOrEmpty
        @($facts.errors | Where-Object { $_ -like 'session_password:*' }).Count | Should -Be 1
    }
}

Describe 'Get-SWLocalPasswordStatus' {
    It 'reads the age and expiry flag of a local account' {
        $account = Get-LocalUser | Where-Object { $null -ne $_.PasswordLastSet } | Select-Object -First 1
        if ($null -eq $account) {
            Set-ItResult -Skipped -Because 'no local account with a password exists on this machine'
            return
        }
        $status = Get-SWLocalPasswordStatus -UserSid $account.SID.Value
        $status.age_seconds | Should -BeGreaterOrEqual 0
        $status.never_expires | Should -BeOfType [bool]
    }
    It 'rejects a value that is not a SID' {
        { Get-SWLocalPasswordStatus -UserSid "S-1-5' OR '1'='1" } | Should -Throw
    }
}
