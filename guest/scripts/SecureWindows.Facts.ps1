#Requires -Version 5.1

<#
.SYNOPSIS
    Collects the guest facts that the plugin uses to select a baseline and protect WinRM.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1. Reads operating system, domain membership,
    server roles, session identity, WinRM configuration and connections, and firewall
    state.

.EXAMPLE
    . .\SecureWindows.Facts.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.1.0
#>


function Get-SWOperatingSystem {
    <#
    .SYNOPSIS
        Collects the Windows version and edition details used to pick a baseline.

    .DESCRIPTION
        Combines the CurrentVersion registry key with Win32_OperatingSystem. Values missing
        from the registry are returned as null or 0, so the plugin can reject an unknown
        build instead of guessing.

    .EXAMPLE
        (Get-SWOperatingSystem).build

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    $currentVersion = Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'
    $os = Get-CimInstance -ClassName Win32_OperatingSystem -ErrorAction Stop
    $optional = {
        param($name)
        $property = $currentVersion.PSObject.Properties[$name]
        if ($null -ne $property) { $property.Value } else { $null }
    }
    [PSCustomObject]@{
        product_name      = & $optional 'ProductName'
        edition_id        = & $optional 'EditionID'
        installation_type = & $optional 'InstallationType'
        display_version   = & $optional 'DisplayVersion'
        major             = [int](& $optional 'CurrentMajorVersionNumber')
        minor             = [int](& $optional 'CurrentMinorVersionNumber')
        build             = [int](& $optional 'CurrentBuildNumber')
        ubr               = [int](& $optional 'UBR')
        product_type      = [int]$os.ProductType
        caption           = [string]$os.Caption
        architecture      = [string]$env:PROCESSOR_ARCHITECTURE
        is_64bit_process  = [Environment]::Is64BitProcess
    }
}

function Get-SWDomainMembership {
    <#
    .SYNOPSIS
        Reports whether the machine is joined to a domain.

    .DESCRIPTION
        Reads Win32_ComputerSystem. The plugin uses the result to choose between the
        domain-joined and non-domain-joined profile.

    .EXAMPLE
        (Get-SWDomainMembership).part_of_domain

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    $computerSystem = Get-CimInstance -ClassName Win32_ComputerSystem -ErrorAction Stop
    [PSCustomObject]@{
        part_of_domain = [bool]$computerSystem.PartOfDomain
        domain         = [string]$computerSystem.Domain
        domain_role    = [int]$computerSystem.DomainRole
    }
}

function Get-SWInstalledRole {
    <#
    .SYNOPSIS
        Lists the installed Windows Server roles.

    .DESCRIPTION
        Returns the sorted names of installed server roles. Returns null on client editions,
        or when Get-WindowsFeature is not available.

    .PARAMETER ProductType
        Win32_OperatingSystem.ProductType: 1 for workstation, 2 for domain controller,
        3 for server.

    .EXAMPLE
        Get-SWInstalledRole -ProductType 3

    .OUTPUTS
        System.String[]

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [int]$ProductType
    )
    if ($ProductType -eq 1 -or -not (Get-Command -Name Get-WindowsFeature -ErrorAction SilentlyContinue)) {
        return $null
    }
    @(Get-WindowsFeature -ErrorAction Stop |
        Where-Object { $_.Installed -and $_.FeatureType -eq 'Role' } |
        ForEach-Object { $_.Name } |
        Sort-Object)
}

function Get-SWSessionIdentity {
    <#
    .SYNOPSIS
        Describes the account and token of the current WinRM session.

    .DESCRIPTION
        Returns the user SID, the group SIDs in the token, and whether the session is a local
        account, the built-in Administrator and elevated. The plugin uses these to work out
        which settings would lock Packer out.

    .EXAMPLE
        (Get-SWSessionIdentity).is_elevated

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent()
    try {
        $principal = New-Object System.Security.Principal.WindowsPrincipal ($identity)
        $groups = @($identity.Groups | ForEach-Object { $_.Value.ToUpperInvariant() } | Sort-Object -Unique)
        $userSid = $identity.User.Value.ToUpperInvariant()
        $isLocal = $groups -contains 'S-1-5-113'
        [PSCustomObject]@{
            user_sid            = $userSid
            user_name           = $identity.Name
            authentication_type = $identity.AuthenticationType
            is_local_account    = $isLocal
            is_builtin_admin    = ($isLocal -and $userSid -match '-500$')
            is_elevated         = $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
            group_sids          = $groups
        }
    }
    finally {
        $identity.Dispose()
    }
}

function ConvertTo-SWBoolean {
    <#
    .SYNOPSIS
        Converts a WSMan provider value to a Boolean.

    .DESCRIPTION
        The WSMan: drive reports settings as the strings 'true' and 'false'. Only 'true'
        (case-insensitive) converts to $true. Anything else, including an empty string,
        converts to $false.

    .PARAMETER Value
        The string value read from the WSMan: drive.

    .EXAMPLE
        ConvertTo-SWBoolean -Value (Get-Item WSMan:\localhost\Service\AllowUnencrypted).Value

    .OUTPUTS
        System.Boolean

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([bool])]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Value
    )
    $Value -eq 'true'
}

function Get-SWWinRMConfiguration {
    <#
    .SYNOPSIS
        Reads the WinRM service, listener and authentication configuration.

    .DESCRIPTION
        Returns the service state, every listener, whether unencrypted traffic is allowed and
        which authentication methods are enabled. The plugin compares this before and after a
        change to confirm that WinRM did not regress.

    .EXAMPLE
        (Get-SWWinRMConfiguration).auth.basic

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    $service = Get-Service -Name WinRM -ErrorAction Stop
    $listeners = foreach ($listener in (Get-ChildItem -Path 'WSMan:\localhost\Listener' -ErrorAction Stop)) {
        $values = @{}
        foreach ($item in (Get-ChildItem -Path "WSMan:\localhost\Listener\$($listener.Name)" -ErrorAction Stop)) {
            $values[$item.Name] = [string]$item.Value
        }
        [PSCustomObject]@{
            transport              = $values['Transport']
            address                = $values['Address']
            port                   = [int]$values['Port']
            enabled                = ConvertTo-SWBoolean -Value $values['Enabled']
            hostname               = $values['Hostname']
            certificate_thumbprint = $values['CertificateThumbprint']
        }
    }
    $auth = @{}
    foreach ($item in (Get-ChildItem -Path 'WSMan:\localhost\Service\Auth' -ErrorAction Stop)) {
        $auth[$item.Name] = [string]$item.Value
    }
    [PSCustomObject]@{
        service_status    = $service.Status.ToString()
        service_start     = $service.StartType.ToString()
        listeners         = @($listeners)
        allow_unencrypted = ConvertTo-SWBoolean -Value ([string](Get-Item -Path 'WSMan:\localhost\Service\AllowUnencrypted').Value)
        auth              = [PSCustomObject]@{
            basic       = ConvertTo-SWBoolean -Value $auth['Basic']
            kerberos    = ConvertTo-SWBoolean -Value $auth['Kerberos']
            negotiate   = ConvertTo-SWBoolean -Value $auth['Negotiate']
            certificate = ConvertTo-SWBoolean -Value $auth['Certificate']
            credssp     = ConvertTo-SWBoolean -Value $auth['CredSSP']
        }
    }
}

function Get-SWWinRMConnection {
    <#
    .SYNOPSIS
        Lists established TCP connections to the WinRM listener ports.

    .DESCRIPTION
        For each connection, returns the local and remote address and the network category of
        the interface carrying it. The plugin uses the category to tell which firewall
        profile carries the Packer session.

    .PARAMETER Port
        The listener ports to match against the local port of each connection.

    .EXAMPLE
        Get-SWWinRMConnection -Port 5985, 5986

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [int[]]$Port
    )
    $connections = Get-NetTCPConnection -State Established -ErrorAction Stop |
    Where-Object { $Port -contains $_.LocalPort }
    foreach ($connection in $connections) {
        $category = $null
        $interfaceIndex = $null
        $address = Get-NetIPAddress -IPAddress $connection.LocalAddress -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $address) {
            $interfaceIndex = [int]$address.InterfaceIndex
            $connectionProfile = Get-NetConnectionProfile -InterfaceIndex $interfaceIndex -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -ne $connectionProfile) {
                $category = $connectionProfile.NetworkCategory.ToString()
            }
        }
        [PSCustomObject]@{
            local_address    = $connection.LocalAddress
            local_port       = [int]$connection.LocalPort
            remote_address   = $connection.RemoteAddress
            interface_index  = $interfaceIndex
            network_category = $category
        }
    }
}

function Get-SWFirewallState {
    <#
    .SYNOPSIS
        Reads the active firewall profiles and the connected network categories.

    .DESCRIPTION
        Returns whether each profile in the active policy store is enabled, its default
        inbound action and whether local rules apply, plus the category of every connected
        network.

    .EXAMPLE
        (Get-SWFirewallState).profiles

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param()
    $profiles = foreach ($fwProfile in (Get-NetFirewallProfile -PolicyStore ActiveStore -ErrorAction Stop)) {
        [PSCustomObject]@{
            name                   = $fwProfile.Name
            enabled                = ($fwProfile.Enabled.ToString() -eq 'True')
            default_inbound_action = $fwProfile.DefaultInboundAction.ToString()
            allow_local_rules      = $fwProfile.AllowLocalFirewallRules.ToString()
        }
    }
    $categories = @(Get-NetConnectionProfile -ErrorAction Stop | ForEach-Object { $_.NetworkCategory.ToString() } | Sort-Object -Unique)
    [PSCustomObject]@{
        profiles                     = @($profiles)
        connected_network_categories = $categories
    }
}

function Get-SWSystemFacts {
    <#
    .SYNOPSIS
        Collects all guest facts for the Detect action.

    .DESCRIPTION
        Gathers operating system, domain, roles, session, WinRM, WinRM connections and
        firewall state. Optional collection failures are added to the errors list so the
        plugin can treat them as unknown and fail closed. Object sections become null;
        connection and role lists become empty arrays. Operating system and session failures
        stop the action. Connections and roles remain arrays when only one item is collected.

    .EXAMPLE
        Write-SWResult -Result (Get-SWSystemFacts)

    .OUTPUTS
        System.Management.Automation.PSCustomObject

    .NOTES
        Author  : Lars Krogh
        Version : 1.1.0
    #>
    [CmdletBinding()]
    param()
    $errors = New-Object System.Collections.Generic.List[string]
    $collect = {
        param($name, [scriptblock]$block)
        try {
            & $block
        }
        catch {
            $errors.Add("${name}: $($_.Exception.Message)")
            $null
        }
    }
    $os = Get-SWOperatingSystem
    $winrm = & $collect 'winrm' { Get-SWWinRMConfiguration }
    $connections = $null
    if ($null -ne $winrm) {
        $ports = @($winrm.listeners | ForEach-Object { $_.port } | Sort-Object -Unique)
        $connections = & $collect 'winrm_connections' { @(Get-SWWinRMConnection -Port $ports) }
    }
    [PSCustomObject]@{
        os                = $os
        domain            = & $collect 'domain' { Get-SWDomainMembership }
        roles             = @(& $collect 'roles' { Get-SWInstalledRole -ProductType $os.product_type } | Where-Object { $null -ne $_ })
        session           = Get-SWSessionIdentity
        winrm             = $winrm
        winrm_connections = @($connections | Where-Object { $null -ne $_ })
        firewall          = & $collect 'firewall' { Get-SWFirewallState }
        errors            = @($errors)
    }

}
