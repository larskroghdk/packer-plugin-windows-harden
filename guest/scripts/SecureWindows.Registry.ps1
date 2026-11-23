#Requires -Version 5.1

<#
.SYNOPSIS
    Reads, writes, removes and restores values in the 64-bit HKLM registry view.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1.
    All registry access goes through an explicit RegistryKey root, so a 32-bit process or WOW64 redirection can never change which view is touched.

.EXAMPLE
    . .\SecureWindows.Registry.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>


function Get-SWRegistryRoot {
    <#
    .SYNOPSIS
        Opens the 64-bit view of HKEY_LOCAL_MACHINE.

    .DESCRIPTION
        Returns HKLM opened with RegistryView.Registry64, so reads and writes never land in the WOW6432Node redirected view.
        The caller must dispose the key.

    .EXAMPLE
        $root = Get-SWRegistryRoot
        try { Get-SWRegistryValue -Root $root -Path 'SOFTWARE\Policies' -Name 'Example' } finally { $root.Dispose() }

    .OUTPUTS
        Microsoft.Win32.RegistryKey

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([Microsoft.Win32.RegistryKey])]
    param()
    [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryView]::Registry64)
}

function ConvertTo-SWUInt32 {
    <#
    .SYNOPSIS
        Reinterprets a signed 32-bit registry DWORD as unsigned.

    .DESCRIPTION
        RegistryKey.GetValue returns REG_DWORD data as Int32, so values above 2147483647 come back negative.
        This keeps the bit pattern and returns the unsigned value that the baseline uses.

    .PARAMETER Value
        The Int32 value read from the registry.

    .EXAMPLE
        ConvertTo-SWUInt32 -Value -1

    .OUTPUTS
        System.UInt32

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([uint32])]
    param(
        [Parameter(Mandatory)]
        [int32]$Value
    )
    [BitConverter]::ToUInt32([BitConverter]::GetBytes($Value), 0)
}

function ConvertTo-SWInt32 {
    <#
    .SYNOPSIS
        Reinterprets an unsigned DWORD as the signed value .NET writes to the registry.

    .DESCRIPTION
        The reverse of ConvertTo-SWUInt32.
        RegistryKey.SetValue takes DWORD data as Int32, so baseline values above 2147483647 have to be passed as their signed equivalent.

    .PARAMETER Value
        The unsigned DWORD value from the baseline.

    .EXAMPLE
        ConvertTo-SWInt32 -Value 4294967295

    .OUTPUTS
        System.Int32

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([int32])]
    param(
        [Parameter(Mandatory)]
        [uint32]$Value
    )
    [BitConverter]::ToInt32([BitConverter]::GetBytes($Value), 0)
}

function Get-SWRegistryValue {
    <#
    .SYNOPSIS
        Reads one registry value below a root key.

    .DESCRIPTION
        Returns present = $false when the key or the value does not exist.
        Otherwise returns the registry type name (REG_DWORD, REG_SZ, ...) and the data as a string.
        DWORDs are returned as unsigned decimal, and environment variables are not expanded.

    .PARAMETER Root
        The open root key, normally from Get-SWRegistryRoot.

    .PARAMETER Path
        The subkey path below the root, without a hive prefix.

    .PARAMETER Name
        The value name. An empty string reads the key's default value.

    .EXAMPLE
        Get-SWRegistryValue -Root $root -Path 'SYSTEM\CurrentControlSet\Control\Lsa' -Name 'LmCompatibilityLevel'

    .OUTPUTS
        System.Management.Automation.PSCustomObject with present, type and value.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [Microsoft.Win32.RegistryKey]$Root,
        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string]$Path,
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Name
    )
    $absent = [PSCustomObject]@{ present = $false; type = $null; value = $null }
    $key = $Root.OpenSubKey($Path, $false)
    if ($null -eq $key) {
        return $absent
    }
    try {
        if ($key.GetValueNames() -notcontains $Name) {
            return $absent
        }
        $kind = $key.GetValueKind($Name)
        $raw = $key.GetValue($Name, $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        switch ($kind) {
            ([Microsoft.Win32.RegistryValueKind]::DWord) {
                $type = 'REG_DWORD'
                $value = [string](ConvertTo-SWUInt32 -Value $raw)
            }
            ([Microsoft.Win32.RegistryValueKind]::String) {
                $type = 'REG_SZ'
                $value = [string]$raw
            }
            ([Microsoft.Win32.RegistryValueKind]::ExpandString) {
                $type = 'REG_EXPAND_SZ'
                $value = [string]$raw
            }
            default {
                $type = "REG_$($kind.ToString().ToUpperInvariant())"
                $value = ($raw | ForEach-Object { [string]$_ }) -join ','
            }
        }
        [PSCustomObject]@{ present = $true; type = $type; value = $value }
    }
    finally {
        $key.Dispose()
    }
}

function Set-SWRegistryValue {
    <#
    .SYNOPSIS
        Writes a REG_DWORD or REG_SZ value, creating the key if needed.

    .DESCRIPTION
        REG_DWORD data must be an unsigned decimal string.
        Anything else, including a negative number, throws before the registry is touched.

    .PARAMETER Root
        The open root key, normally from Get-SWRegistryRoot.

    .PARAMETER Path
        The subkey path below the root, without a hive prefix.

    .PARAMETER Name
        The value name.

    .PARAMETER Type
        REG_DWORD or REG_SZ.

    .PARAMETER Data
        The value data as a string.

    .EXAMPLE
        Set-SWRegistryValue -Root $root -Path 'SYSTEM\CurrentControlSet\Control\Lsa' -Name 'LmCompatibilityLevel' -Type REG_DWORD -Data '5'

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [Microsoft.Win32.RegistryKey]$Root,
        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string]$Path,
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Name,
        [Parameter(Mandatory)]
        [ValidateSet('REG_DWORD', 'REG_SZ')]
        [string]$Type,
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Data
    )
    $key = $Root.CreateSubKey($Path, $true)
    if ($null -eq $key) {
        throw "Unable to open or create registry key HKLM\$Path"
    }
    try {
        if ($Type -eq 'REG_DWORD') {
            $number = [uint32]::Parse($Data, [Globalization.CultureInfo]::InvariantCulture)
            $key.SetValue($Name, (ConvertTo-SWInt32 -Value $number), [Microsoft.Win32.RegistryValueKind]::DWord)
        }
        else {
            $key.SetValue($Name, $Data, [Microsoft.Win32.RegistryValueKind]::String)
        }
    }
    finally {
        $key.Dispose()
    }
}

function Remove-SWRegistryValue {
    <#
    .SYNOPSIS
        Deletes a registry value if it exists.

    .DESCRIPTION
        Does nothing when the key or the value is already absent.
        The key itself is left in place.

    .PARAMETER Root
        The open root key, normally from Get-SWRegistryRoot.

    .PARAMETER Path
        The subkey path below the root, without a hive prefix.

    .PARAMETER Name
        The value name to delete.

    .EXAMPLE
        Remove-SWRegistryValue -Root $root -Path 'SOFTWARE\Policies\Microsoft\Windows\System' -Name 'EnumerateLocalUsers'

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [Microsoft.Win32.RegistryKey]$Root,
        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string]$Path,
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Name
    )
    $key = $Root.OpenSubKey($Path, $true)
    if ($null -eq $key) {
        return
    }
    try {
        $key.DeleteValue($Name, $false)
    }
    finally {
        $key.Dispose()
    }
}

function Restore-SWRegistryValue {
    <#
    .SYNOPSIS
        Puts a registry value back to a previously recorded state.

    .DESCRIPTION
        Deletes the value when the recorded state is absent.
        Otherwise writes the recorded type and data.
        Throws for any type other than REG_DWORD or REG_SZ, because those are the only types the plugin writes.

    .PARAMETER Root
        The open root key, normally from Get-SWRegistryRoot.

    .PARAMETER Path
        The subkey path below the root, without a hive prefix.

    .PARAMETER Name
        The value name.

    .PARAMETER State
        The recorded state, with present, type and value, as returned by Get-SWRegistryValue.

    .EXAMPLE
        Restore-SWRegistryValue -Root $root -Path $registry.path -Name $registry.name -State $item.state

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [Microsoft.Win32.RegistryKey]$Root,
        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string]$Path,
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Name,
        [Parameter(Mandatory)]
        [PSCustomObject]$State
    )
    if (-not $State.present) {
        Remove-SWRegistryValue -Root $Root -Path $Path -Name $Name
        return
    }
    if ($State.type -notin 'REG_DWORD', 'REG_SZ') {
        throw "Cannot restore HKLM\$Path\$Name with type $($State.type)"
    }
    Set-SWRegistryValue -Root $Root -Path $Path -Name $Name -Type $State.type -Data $State.value
}
