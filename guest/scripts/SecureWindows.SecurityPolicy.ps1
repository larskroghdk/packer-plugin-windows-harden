#Requires -Version 5.1

<#
.SYNOPSIS
    Reads and writes user rights and account policies through secedit.exe.

.DESCRIPTION
    Dot-sourced by Invoke-SecureWindows.ps1. Exports and parses security templates and
    applies changes from a generated template. Requires Invoke-SWNativeCommand from
    SecureWindows.NativeCommand.ps1.

.EXAMPLE
    . .\SecureWindows.SecurityPolicy.ps1

.NOTES
    Author  : Lars Krogh
    Version : 1.0.0
#>


function ConvertFrom-SWSecurityTemplate {
    <#
    .SYNOPSIS
        Parses a secedit security template into sections of key/value pairs.

    .DESCRIPTION
        Returns a hashtable keyed by section name. Each section is a case-insensitive
        hashtable of its entries. Blank lines, ';' comments, entries before the first section
        and lines without a key are ignored.

    .PARAMETER Text
        The template text, already decoded from UTF-16.

    .EXAMPLE
        ConvertFrom-SWSecurityTemplate -Text ([IO.File]::ReadAllText($path, [Text.Encoding]::Unicode))

    .OUTPUTS
        System.Collections.Hashtable

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([hashtable])]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Text
    )
    $sections = @{}
    $current = $null
    foreach ($line in ($Text -split '\r?\n')) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith(';')) {
            continue
        }
        if ($trimmed -match '^\[(.+)\]$') {
            $current = New-Object System.Collections.Hashtable ([StringComparer]::OrdinalIgnoreCase)
            $sections[$Matches[1]] = $current
            continue
        }
        if ($null -eq $current) {
            continue
        }
        $index = $trimmed.IndexOf('=')
        if ($index -lt 1) {
            continue
        }
        $current[$trimmed.Substring(0, $index).Trim()] = $trimmed.Substring($index + 1).Trim()
    }
    $sections
}

function ConvertTo-SWSidList {
    <#
    .SYNOPSIS
        Normalizes a user-right assignment to a sorted list of SIDs.

    .DESCRIPTION
        Accepts the comma-separated principals from a security template. Entries prefixed
        with '*' are SIDs. Other entries are account names, which are translated to SIDs and
        kept as-is if they cannot be resolved. Returns upper-case SIDs, sorted and
        de-duplicated, joined with commas.

    .PARAMETER Value
        The assignment string, for example '*S-1-5-32-544,*S-1-5-11'.

    .EXAMPLE
        ConvertTo-SWSidList -Value '*S-1-5-32-555,*S-1-5-32-544'

    .OUTPUTS
        System.String

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([string])]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Value
    )
    $sids = foreach ($item in ($Value -split ',')) {
        $principal = $item.Trim()
        if ($principal -eq '') {
            continue
        }
        if ($principal.StartsWith('*')) {
            $principal.Substring(1).ToUpperInvariant()
            continue
        }
        try {
            $account = New-Object System.Security.Principal.NTAccount ($principal)
            $account.Translate([System.Security.Principal.SecurityIdentifier]).Value.ToUpperInvariant()
        }
        catch [System.Security.Principal.IdentityNotMappedException] {
            $principal
        }
    }
    (@($sids) | Sort-Object -Unique) -join ','
}

function Invoke-SWSecedit {
    <#
    .SYNOPSIS
        Runs secedit.exe quietly and checks its exit code.

    .DESCRIPTION
        Adds /log and /quiet to the arguments. Exit code 3 means secedit finished with
        warnings and is returned to the caller. Any other non-zero code throws, with
        secedit's output and the last lines of its log.

    .PARAMETER ArgumentList
        The secedit arguments, starting with the operation (/export or /configure).

    .PARAMETER LogPath
        Where secedit writes its log.

    .EXAMPLE
        Invoke-SWSecedit -ArgumentList @('/export', '/cfg', $templatePath, '/areas', 'USER_RIGHTS') -LogPath $logPath

    .OUTPUTS
        System.Int32

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([int])]
    param(
        [Parameter(Mandatory)]
        [string[]]$ArgumentList,
        [Parameter(Mandatory)]
        [string]$LogPath
    )
    $secedit = Join-Path -Path $env:SystemRoot -ChildPath 'System32\secedit.exe'
    $result = Invoke-SWNativeCommand -FilePath $secedit -ArgumentList ($ArgumentList + @('/log', $LogPath, '/quiet'))
    if ($result.ExitCode -ne 0 -and $result.ExitCode -ne 3) {
        $tail = ''
        if (Test-Path -LiteralPath $LogPath -PathType Leaf) {
            $tail = (Get-Content -LiteralPath $LogPath -Tail 20) -join ' | '
        }
        throw "secedit.exe $($ArgumentList[0]) failed with exit code $($result.ExitCode). Output: $($result.Output) Log: $tail"
    }
    $result.ExitCode
}

function Export-SWSecurityPolicy {
    <#
    .SYNOPSIS
        Exports the local security policy and user rights.

    .DESCRIPTION
        Runs secedit /export for the SECURITYPOLICY and USER_RIGHTS areas, parses the
        template and deletes the exported file.

    .PARAMETER WorkPath
        A writable directory for the exported template and the secedit log.

    .EXAMPLE
        $policy = Export-SWSecurityPolicy -WorkPath $workPath

    .OUTPUTS
        System.Collections.Hashtable

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([hashtable])]
    param(
        [Parameter(Mandatory)]
        [string]$WorkPath
    )
    $templatePath = Join-Path -Path $WorkPath -ChildPath 'secedit-export.inf'
    $logPath = Join-Path -Path $WorkPath -ChildPath 'secedit-export.log'
    $null = Invoke-SWSecedit -ArgumentList @('/export', '/cfg', $templatePath, '/areas', 'SECURITYPOLICY', 'USER_RIGHTS') -LogPath $logPath
    try {
        ConvertFrom-SWSecurityTemplate -Text ([IO.File]::ReadAllText($templatePath, [Text.Encoding]::Unicode))
    }
    finally {
        Remove-Item -LiteralPath $templatePath -Force -ErrorAction SilentlyContinue
    }
}

function Get-SWUserRight {
    <#
    .SYNOPSIS
        Reads who holds a user right in an exported security policy.

    .DESCRIPTION
        A right that does not appear in the export is assigned to no one. It is reported as
        present with an empty value, not as absent.

    .PARAMETER Policy
        The policy returned by Export-SWSecurityPolicy.

    .PARAMETER Right
        The privilege or logon right constant, for example SeNetworkLogonRight.

    .EXAMPLE
        Get-SWUserRight -Policy $policy -Right 'SeDenyNetworkLogonRight'

    .OUTPUTS
        System.Management.Automation.PSCustomObject with present, type and value.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param (
        [Parameter(Mandatory)]
        [hashtable]$Policy,
        [Parameter(Mandatory)]
        [string]$Right
    )
    $value = ''
    if ($Policy.ContainsKey('Privilege Rights') -and $Policy['Privilege Rights'].ContainsKey($Right)) {
        $value = $Policy['Privilege Rights'][$Right]
    }
    [PSCustomObject]@{ present = $true; type = $null; value = (ConvertTo-SWSidList -Value $value) }
}

function Get-SWAccountPolicy {
    <#
    .SYNOPSIS
        Reads an account policy value from an exported security policy.

    .DESCRIPTION
        Looks the name up in the [System Access] section. A name that is not exported is
        reported as absent.

    .PARAMETER Policy
        The policy returned by Export-SWSecurityPolicy.

    .PARAMETER Name
        The [System Access] key, for example MinimumPasswordLength.

    .EXAMPLE
        Get-SWAccountPolicy -Policy $policy -Name 'LockoutBadCount'

    .OUTPUTS
        System.Management.Automation.PSCustomObject with present, type and value.

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [hashtable]$Policy,
        [Parameter(Mandatory)]
        [string]$Name
    )
    if ($Policy.ContainsKey('System Access') -and $Policy['System Access'].ContainsKey($Name)) {
        return [PSCustomObject]@{ present = $true; type = $null; value = [string]$Policy['System Access'][$Name] }
    }
    [PSCustomObject]@{ present = $false; type = $null; value = $null }
}

function Set-SWSecurityPolicy {
    <#
    .SYNOPSIS
        Applies user rights and account policies in one secedit run.

    .DESCRIPTION
        Writes a UTF-16 security template with only the given entries and configures it into
        a throw-away database, limited to the areas that have entries. The template, the
        database and its journal are deleted afterwards. Returns 0 without running secedit
        when there is nothing to apply.

    .PARAMETER WorkPath
        A writable directory for the template, the database and the log.

    .PARAMETER UserRights
        Right names mapped to the SIDs that should hold them. An empty list assigns the right
        to no one.

    .PARAMETER AccountPolicies
        [System Access] keys mapped to their values.

    .EXAMPLE
        Set-SWSecurityPolicy -WorkPath $workPath -UserRights @{ SeDenyNetworkLogonRight = @('S-1-5-114') }

    .OUTPUTS
        System.Int32

    .NOTES
        Author  : Lars Krogh
        Version : 1.0.0
    #>
    [CmdletBinding()]
    [OutputType([int])]
    param(
        [Parameter(Mandatory)]
        [string]$WorkPath,
        [Parameter()]
        [hashtable]$UserRights = @{},
        [Parameter()]
        [hashtable]$AccountPolicies = @{}
    )
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('[Unicode]')
    $lines.Add('Unicode=yes')
    $lines.Add('[Version]')
    $lines.Add('signature="$CHICAGO$"')
    $lines.Add('Revision=1')
    $areas = New-Object System.Collections.Generic.List[string]
    if ($AccountPolicies.Count -gt 0) {
        $lines.Add('[System Access]')
        foreach ($name in ($AccountPolicies.Keys | Sort-Object)) {
            $lines.Add("$name = $($AccountPolicies[$name])")
        }
        $areas.Add('SECURITYPOLICY')
    }
    if ($UserRights.Count -gt 0) {
        $lines.Add('[Privilege Rights]')
        foreach ($right in ($UserRights.Keys | Sort-Object)) {
            $principals = @($UserRights[$right] | ForEach-Object { "*$_" }) -join ','
            $lines.Add("$right = $principals")
        }
        $areas.Add('USER_RIGHTS')
    }
    if ($areas.Count -eq 0) {
        return 0
    }
    $templatePath = Join-Path -Path $WorkPath -ChildPath 'secedit-apply.inf'
    $databasePath = Join-Path -Path $WorkPath -ChildPath ('secedit-{0}.sdb' -f [guid]::NewGuid().ToString('N'))
    $logPath = Join-Path -Path $WorkPath -ChildPath 'secedit-apply.log'
    [IO.File]::WriteAllText($templatePath, (($lines -join "`r`n") + "`r`n"), [Text.Encoding]::Unicode)
    try {
        $arguments = @('/configure', '/db', $databasePath, '/cfg', $templatePath, '/areas') + $areas
        Invoke-SWSecedit -ArgumentList $arguments -LogPath $logPath
    }
    finally {
        Remove-Item -LiteralPath $templatePath, $databasePath, "$databasePath.jfm" -Force -ErrorAction SilentlyContinue
    }

}
