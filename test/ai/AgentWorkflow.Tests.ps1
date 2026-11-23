#Requires -Version 7.2

<#
.SYNOPSIS
    Pester tests for the agent workflow scripts and Git hooks.

.DESCRIPTION
    Each test builds a throwaway Git repository in TestDrive with a copy of scripts/ai and .githooks, settings with one cheap check, and FakeAgent.ps1 standing in for codex, gemini and copilot on PATH.
    The tests run real git commits and pushes against a local bare remote, so the hooks, review records, approval policy and failure handling are exercised end to end without network access or real agent calls.

.EXAMPLE
    Invoke-Pester -Path test/ai

.NOTES
    Author  : Lars Krogh
    Version : 1.1.0
#>


BeforeAll {
    $script:sourceRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
    $script:fakeAgent = Join-Path $PSScriptRoot 'FakeAgent.ps1'
    $script:pwshPath = (Get-Process -Id $PID).Path
    $script:savedPath = $env:PATH
    Remove-Item Env:AI_AGENT_ACTIVE -ErrorAction SilentlyContinue
    $script:fakeBin = Join-Path $TestDrive 'bin'
    $null = New-Item -ItemType Directory -Path $fakeBin
    foreach ($name in 'codex', 'gemini', 'copilot') {
        if ($IsWindows) {
            Set-Content -LiteralPath (Join-Path $fakeBin "$name.cmd") -Value "@`"$pwshPath`" -NoProfile -NonInteractive -File `"$fakeAgent`" $name %*" -Encoding ascii
        }
        else {
            $path = Join-Path $fakeBin $name
            Set-Content -LiteralPath $path -Value "#!/bin/sh`nexec `"$pwshPath`" -NoProfile -NonInteractive -File `"$fakeAgent`" $name `"`$@`"" -Encoding ascii
            & chmod +x $path
        }
    }
    $env:PATH = $fakeBin + [IO.Path]::PathSeparator + $env:PATH

    function Invoke-Git {
        <#
        .SYNOPSIS
            Runs git in the test repository and returns its output and exit code.

        .DESCRIPTION
            Combines standard output and error so hook messages can be asserted.

        .PARAMETER GitArguments
            The git arguments, passed unbound so that options such as -A reach git.

        .EXAMPLE
            Invoke-Git commit -m 'x'

        .OUTPUTS
            System.Management.Automation.PSCustomObject with ExitCode and Output.

        .NOTES
            Author  : Lars Krogh
            Version : 1.0.0
        #>
        $gitArguments = @($args)
        $output = & git -C $script:repo @gitArguments 2>&1 | Out-String
        [PSCustomObject]@{ ExitCode = $LASTEXITCODE; Output = $output }
    }

    function Initialize-TestRepository {
        <#
        .SYNOPSIS
            Creates a repository with the workflow scripts, hooks and test settings.

        .DESCRIPTION
            Copies scripts/ai and .githooks from the source tree, writes settings that use the fake agents and one check that fails when a file named FAIL exists, makes an initial commit without hooks, installs the hooks and adds a bare remote.

        .PARAMETER CodexCommand
            The Codex command name in the settings.

        .EXAMPLE
            Initialize-TestRepository

        .NOTES
            Author  : Lars Krogh
            Version : 1.0.0
        #>
        param(
            [string]$CodexCommand = 'codex'
        )
        $script:repo = Join-Path $TestDrive ([guid]::NewGuid().ToString('N'))
        $remote = "$repo-remote.git"
        $null = New-Item -ItemType Directory -Path (Join-Path $repo 'scripts/ai'), (Join-Path $repo '.githooks')
        Copy-Item -Path (Join-Path $sourceRoot 'scripts/ai/*') -Destination (Join-Path $repo 'scripts/ai')
        Copy-Item -Path (Join-Path $sourceRoot '.githooks/*') -Destination (Join-Path $repo '.githooks')
        if (-not $IsWindows) {
            & chmod +x @(Get-ChildItem -LiteralPath (Join-Path $repo '.githooks') -File | ForEach-Object FullName)
        }
        $settings = @"
@{
    Codex = @{ Command = '$CodexCommand'; Model = ''; TimeoutSeconds = 20; MaxAttempts = 1; MaxDiffBytes = 100000; BlockingSeverities = @('critical', 'high') }
    Gemini = @{ Command = 'gemini'; Model = ''; TimeoutSeconds = 30; MaxAttempts = 1; CacheDays = 7; InputUsdPerMillionTokens = 1; OutputUsdPerMillionTokens = 10 }
    Copilot = @{ Command = 'copilot'; Model = ''; TimeoutSeconds = 30; MaxAiCredits = 0; WriteRules = @('*.md', 'docs/**'); AllowedPathPatterns = @('*.md', 'docs/*'); DenyTools = @('shell(git commit)', 'shell(git push)') }
    CheckTimeoutSeconds = 60
    Checks = @(
        @{ Name = 'fail marker'; Paths = @('*'); Command = @('pwsh', '-NoProfile', '-Command', 'if (Test-Path FAIL) { exit 1 }') }
    )
}
"@
        Set-Content -LiteralPath (Join-Path $repo 'scripts/ai/settings.psd1') -Value $settings
        Set-Content -LiteralPath (Join-Path $repo 'main.txt') -Value 'initial'
        $null = & git init -q -b main $repo
        $null = Invoke-Git config user.name 'Test'
        $null = Invoke-Git config user.email 'test@example.invalid'
        $null = Invoke-Git config core.autocrlf false
        $null = Invoke-Git add -A
        $null = Invoke-Git commit -q --no-verify -m 'initial'
        $null = Invoke-Git config core.hooksPath .githooks
        $null = & git init -q --bare $remote
        $null = Invoke-Git remote add origin $remote
        $null = Invoke-Git push -q --no-verify origin main
        $script:log = Join-Path $TestDrive "$([guid]::NewGuid().ToString('N')).log"
        $env:FAKE_AGENT_LOG = $log
    }

    function Add-StagedChange {
        <#
        .SYNOPSIS
            Writes a file in the test repository and stages it.

        .DESCRIPTION
            Creates or replaces the file and runs git add for it.

        .PARAMETER Name
            The file name.

        .PARAMETER Content
            The file content.

        .EXAMPLE
            Add-StagedChange -Name main.txt -Content 'changed'

        .NOTES
            Author  : Lars Krogh
            Version : 1.0.0
        #>
        param(
            [Parameter(Mandatory)]
            [string]$Name,
            [string]$Content = 'changed'
        )
        Set-Content -LiteralPath (Join-Path $repo $Name) -Value $Content
        $null = Invoke-Git add -- $Name
    }

    function Get-AgentCall {
        <#
        .SYNOPSIS
            Returns the logged calls of one fake agent.

        .DESCRIPTION
            Reads FAKE_AGENT_LOG and keeps the lines of the agent, ignoring --version probes.

        .PARAMETER Agent
            codex, gemini or copilot.

        .EXAMPLE
            Get-AgentCall -Agent codex

        .OUTPUTS
            System.String

        .NOTES
            Author  : Lars Krogh
            Version : 1.0.0
        #>
        param(
            [Parameter(Mandatory)]
            [string]$Agent
        )
        if (-not (Test-Path -LiteralPath $log)) {
            return , @()
        }
        Write-Output -NoEnumerate -InputObject @(Get-Content -LiteralPath $log | Where-Object { $_ -like "$Agent *" -and $_ -notlike '* --version' -and $_ -notlike '* login status' })
    }

    function Invoke-Script {
        <#
        .SYNOPSIS
            Runs a workflow script in the test repository.

        .DESCRIPTION
            Starts a new pwsh process in the repository, as Claude or a hook would, and returns its exit code and output.

        .PARAMETER ScriptArguments
            The script file name in scripts/ai, followed by its arguments, passed unbound.

        .EXAMPLE
            Invoke-Script Invoke-CodexReview.ps1 -Commit HEAD

        .OUTPUTS
            System.Management.Automation.PSCustomObject with ExitCode and Output.

        .NOTES
            Author  : Lars Krogh
            Version : 1.0.0
        #>
        $name = $args[0]
        $scriptArguments = @($args | Select-Object -Skip 1)
        Push-Location $repo
        try {
            $output = & $pwshPath -NoProfile -NonInteractive -File (Join-Path $repo "scripts/ai/$name") @scriptArguments 2>&1 | Out-String
            [PSCustomObject]@{ ExitCode = $LASTEXITCODE; Output = $output }
        }
        finally {
            Pop-Location
        }
    }
}

AfterAll {
    $env:PATH = $savedPath
    Remove-Item Env:FAKE_AGENT_LOG, Env:FAKE_CODEX_MODE, Env:FAKE_GEMINI_MODE, Env:FAKE_COPILOT_MODE, Env:AI_AGENT_ACTIVE -ErrorAction SilentlyContinue
}

Describe 'Pre-commit Codex gate' {
    BeforeEach {
        Initialize-TestRepository
        $env:FAKE_CODEX_MODE = 'approve'
    }
    It 'allows a commit that Codex approves and reviews it in a read-only sandbox' {
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Git commit -m 'approved change'
        $result.ExitCode | Should -Be 0 -Because $result.Output
        $result.Output | Should -Match 'Commit allowed'
        $calls = Get-AgentCall -Agent codex
        $calls.Count | Should -Be 1
        $calls[0] | Should -Match 'exec --sandbox read-only --ephemeral'
        (Invoke-Git log --oneline).Output | Should -Match 'approved change'
    }
    It 'blocks a commit that Codex rejects and shows the findings' {
        $env:FAKE_CODEX_MODE = 'reject'
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Git commit -m 'rejected change'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'REJECTED'
        $result.Output | Should -Match '\[HIGH\] main.txt:1 Fake defect'
        (Invoke-Git log --oneline).Output | Should -Not -Match 'rejected change'
    }
    It 'blocks when Codex approves despite a high finding' {
        $env:FAKE_CODEX_MODE = 'approve-with-high'
        Add-StagedChange -Name 'main.txt'
        (Invoke-Git commit -m 'policy').ExitCode | Should -Not -Be 0
    }
    It 'fails closed when Codex <Mode>' -ForEach @(
        @{ Mode = 'fail'; Message = 'exited with code 1' }
        @{ Mode = 'malformed'; Message = 'does not match review-schema.json' }
        @{ Mode = 'wrong-type'; Message = 'does not match review-schema.json' }
    ) {
        $env:FAKE_CODEX_MODE = $Mode
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Git commit -m 'unreviewed'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match $Message
        $result.Output | Should -Match 'not approved'
    }
    It 'reads the checks from the staged settings, not from unstaged edits' {
        Add-StagedChange -Name 'FAIL'
        $settings = Join-Path $repo 'scripts/ai/settings.psd1'
        @(Get-Content -LiteralPath $settings) | Where-Object { $_ -notmatch 'fail marker' } | Set-Content -LiteralPath $settings
        $result = Invoke-Git commit -m 'unstaged policy'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match "Check 'fail marker' failed"
        Get-AgentCall -Agent codex | Should -BeNullOrEmpty
    }
    It 'blocks the commit when the index changes during the review' {
        $env:FAKE_CODEX_MODE = 'restage'
        $env:FAKE_REPO = $repo
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Script Invoke-AgentWorkflow.ps1 -Stage PreCommit
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'staged changes changed during the review'
    }
    It 'fails closed when Codex times out' {
        $env:FAKE_CODEX_MODE = 'hang'
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Git commit -m 'timeout'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'timed out'
    }
    It 'blocks the commit when Codex is not installed' {
        Initialize-TestRepository -CodexCommand 'codex-not-installed'
        Add-StagedChange -Name 'main.txt'
        $result = Invoke-Git commit -m 'no reviewer'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'codex-not-installed is not installed'
    }
    It 'runs the checks first and does not ask Codex when they fail' {
        Add-StagedChange -Name 'FAIL'
        $result = Invoke-Git commit -m 'failing check'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match "Check 'fail marker' failed"
        (Get-AgentCall -Agent codex).Count | Should -Be 0
    }
    It 'reviews the staged content, not unstaged edits' {
        Add-StagedChange -Name 'main.txt' -Content 'staged'
        Set-Content -LiteralPath (Join-Path $repo 'FAIL') -Value 'untracked'
        (Invoke-Git commit -m 'staged only').ExitCode | Should -Be 0
    }
    It 'reuses an approval only for the identical changeset' {
        Add-StagedChange -Name 'main.txt' -Content 'first'
        (Invoke-Script Invoke-CodexReview.ps1).ExitCode | Should -Be 0
        $env:FAKE_CODEX_MODE = 'fail'
        (Invoke-Script Invoke-CodexReview.ps1).Output | Should -Match 'already approved'
        Add-StagedChange -Name 'main.txt' -Content 'second'
        $result = Invoke-Git commit -m 'changed after approval'
        $result.ExitCode | Should -Not -Be 0
        (Get-AgentCall -Agent codex).Count | Should -Be 2
        Add-StagedChange -Name 'main.txt' -Content 'first'
        (Invoke-Git commit -m 'approved content').ExitCode | Should -Be 0
        (Get-AgentCall -Agent codex).Count | Should -Be 2
    }
    It 'refuses to run when started by an agent process' {
        Add-StagedChange -Name 'main.txt'
        $env:AI_AGENT_ACTIVE = '1'
        try {
            $result = Invoke-Git commit -m 'nested'
        }
        finally {
            Remove-Item Env:AI_AGENT_ACTIVE
        }
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'started by an agent process'
        (Get-AgentCall -Agent codex).Count | Should -Be 0
    }
    It 'refuses while another agent holds the write lock' {
        . (Join-Path $repo 'scripts/ai/AgentTools.ps1')
        $lock = Enter-AIWriteLock -RepositoryRoot $repo
        try {
            Add-StagedChange -Name 'main.txt'
            $result = Invoke-Git commit -m 'during copilot'
        }
        finally {
            $lock.Dispose()
        }
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'Another agent is changing files'
    }
}

Describe 'Process runner' {
    BeforeAll {
        . (Join-Path $sourceRoot 'scripts/ai/AgentTools.ps1')
    }
    It 'kills a process that never reads its input when the timeout passes' {
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $result = Invoke-AIProcess -FilePath $pwshPath -ArgumentList '-NoProfile', '-Command', 'Start-Sleep -Seconds 60' -InputText ('x' * 8MB) -TimeoutSeconds 2
        $watch.Stop()
        $result.TimedOut | Should -BeTrue
        $result.ExitCode | Should -BeNullOrEmpty
        $watch.Elapsed.TotalSeconds | Should -BeLessThan 30
    }
    It 'runs a batch file through cmd.exe with its arguments intact' -Skip:(-not $IsWindows) {
        $batch = Join-Path $TestDrive 'echo args.cmd'
        Set-Content -LiteralPath $batch -Value '@echo %*' -Encoding ascii
        $result = Invoke-AIProcess -FilePath $batch -ArgumentList 'two words', 'write(*.md)', 'a&b|c' -TimeoutSeconds 30
        $result.ExitCode | Should -Be 0
        $result.StandardOutput.Trim() | Should -Be '"two words" "write(*.md)" "a&b|c"'
    }
    It 'refuses batch file arguments that cmd.exe would expand' -Skip:(-not $IsWindows) {
        $batch = Join-Path $TestDrive 'noop.cmd'
        Set-Content -LiteralPath $batch -Value '@exit /b 0' -Encoding ascii
        { Invoke-AIProcess -FilePath $batch -ArgumentList '%PATH%' -TimeoutSeconds 30 } | Should -Throw '*cmd.exe would interpret*'
        { Invoke-AIProcess -FilePath $batch -ArgumentList 'say "hi"' -TimeoutSeconds 30 } | Should -Throw '*cmd.exe would interpret*'
    }
}

Describe 'Pre-push verification' {
    BeforeEach {
        Initialize-TestRepository
        $env:FAKE_CODEX_MODE = 'approve'
    }
    It 'blocks a push of a commit made without review, and allows it after a review' {
        Add-StagedChange -Name 'main.txt'
        $null = Invoke-Git commit -q --no-verify -m 'skipped hook'
        $blocked = Invoke-Git push origin main
        $blocked.ExitCode | Should -Not -Be 0
        $blocked.Output | Should -Match 'not approved'
        (Invoke-Script Invoke-CodexReview.ps1 -Commit HEAD).ExitCode | Should -Be 0
        $allowed = Invoke-Git push origin main
        $allowed.ExitCode | Should -Be 0 -Because $allowed.Output
        $allowed.Output | Should -Match 'every outgoing commit is approved'
    }
    It 'allows a push of an amended commit when the commit and the amendment were both approved' {
        Add-StagedChange -Name 'main.txt' -Content 'first'
        (Invoke-Git commit -q -m 'first').ExitCode | Should -Be 0
        Add-StagedChange -Name 'main.txt' -Content 'amended'
        (Invoke-Git commit -q --amend -m 'amended').ExitCode | Should -Be 0
        $pushed = Invoke-Git push origin main
        $pushed.ExitCode | Should -Be 0 -Because $pushed.Output
        (Get-AgentCall -Agent codex).Count | Should -Be 2
    }
    It 'ignores malformed unrelated review records' {
        $reviews = Join-Path $repo '.git/ai/reviews'
        $null = New-Item -ItemType Directory -Force -Path $reviews
        Set-Content -LiteralPath (Join-Path $reviews 'null.json') -Value 'null'
        Set-Content -LiteralPath (Join-Path $reviews 'broken.json') -Value '{ not json'
        Set-Content -LiteralPath (Join-Path $reviews 'a-b.json') -Value '{"changeset":"a-b","base_tree":"a","tree":"b"}'
        Add-StagedChange -Name 'main.txt'
        (Invoke-Git commit -q -m 'reviewed').ExitCode | Should -Be 0
        $pushed = Invoke-Git push origin main
        $pushed.ExitCode | Should -Be 0 -Because $pushed.Output
    }
    It 'reviews an automatic merge commit and allows pushing it' {
        $null = Invoke-Git switch -q -c feature
        Add-StagedChange -Name 'feature.txt'
        (Invoke-Git commit -q -m 'feature').ExitCode | Should -Be 0
        $null = Invoke-Git switch -q main
        Add-StagedChange -Name 'main.txt'
        (Invoke-Git commit -q -m 'main').ExitCode | Should -Be 0
        $merged = Invoke-Git merge --no-ff -m 'merge feature' feature
        $merged.ExitCode | Should -Be 0 -Because $merged.Output
        $merged.Output | Should -Match 'Commit allowed'
        $pushed = Invoke-Git push origin main
        $pushed.ExitCode | Should -Be 0 -Because $pushed.Output
    }
    It 'blocks a push of a merge commit that was not reviewed' {
        $null = Invoke-Git switch -q -c feature
        Add-StagedChange -Name 'feature.txt'
        (Invoke-Git commit -q -m 'feature').ExitCode | Should -Be 0
        $null = Invoke-Git switch -q main
        Add-StagedChange -Name 'main.txt'
        (Invoke-Git commit -q -m 'main').ExitCode | Should -Be 0
        $null = Invoke-Git merge --no-ff --no-verify -m 'unreviewed merge' feature
        $blocked = Invoke-Git push origin main
        $blocked.ExitCode | Should -Not -Be 0
        $blocked.Output | Should -Match 'not approved'
    }
    It 'blocks a push of an amended commit when the original commit was not reviewed' {
        Add-StagedChange -Name 'main.txt' -Content 'first'
        $null = Invoke-Git commit -q --no-verify -m 'first'
        Add-StagedChange -Name 'main.txt' -Content 'amended'
        (Invoke-Git commit -q --amend -m 'amended').ExitCode | Should -Be 0
        $blocked = Invoke-Git push origin main
        $blocked.ExitCode | Should -Not -Be 0
        $blocked.Output | Should -Match 'not approved'
    }
    It 'blocks a push of a new branch with a commit whose review was rejected' {
        $null = Invoke-Git switch -q -c feature
        $env:FAKE_CODEX_MODE = 'reject'
        Add-StagedChange -Name 'main.txt'
        $null = Invoke-Git commit -q --no-verify -m 'rejected'
        $null = Invoke-Script Invoke-CodexReview.ps1 -Commit HEAD
        (Invoke-Git push origin feature).ExitCode | Should -Not -Be 0
    }
}

Describe 'Gemini research' {
    BeforeEach {
        Initialize-TestRepository
        $env:FAKE_GEMINI_MODE = 'answer'
    }
    It 'runs read-only in plan mode with JSON output, reports cost and caches the answer' {
        $first = Invoke-Script Invoke-GeminiResearch.ps1 -Question 'How is WinRM protected?'
        $first.ExitCode | Should -Be 0 -Because $first.Output
        $first.Output | Should -Match 'Fake research answer'
        $first.Output | Should -Match '1000 input and 250 output tokens'
        $first.Output | Should -Match 'estimated USD 0.0035'
        $calls = Get-AgentCall -Agent gemini
        $calls.Count | Should -Be 1
        $calls[0] | Should -Match '--approval-mode plan -o json'
        $calls[0] | Should -Not -Match 'WinRM'
        $second = Invoke-Script Invoke-GeminiResearch.ps1 -Question 'How is WinRM protected?'
        $second.Output | Should -Match 'Cached Gemini answer'
        (Get-AgentCall -Agent gemini).Count | Should -Be 1
    }
    It 'asks again when the working tree changed after the cached answer' {
        (Invoke-Script Invoke-GeminiResearch.ps1 -Question 'What does main.txt hold?').ExitCode | Should -Be 0
        Set-Content -LiteralPath (Join-Path $repo 'notes.txt') -Value 'uncommitted edit'
        $again = Invoke-Script Invoke-GeminiResearch.ps1 -Question 'What does main.txt hold?'
        $again.ExitCode | Should -Be 0 -Because $again.Output
        $again.Output | Should -Not -Match 'Cached Gemini answer'
        (Get-AgentCall -Agent gemini).Count | Should -Be 2
    }
    It 'fails when Gemini changes a file' {
        $env:FAKE_GEMINI_MODE = 'modify'
        $result = Invoke-Script Invoke-GeminiResearch.ps1 -Question 'Anything'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'changed files during read-only research: gemini-was-here.txt'
    }
    It 'fails without an answer when Gemini reports an API error' {
        $env:FAKE_GEMINI_MODE = 'fail'
        $result = Invoke-Script Invoke-GeminiResearch.ps1 -Question 'Anything'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'API key not valid'
    }
}

Describe 'Copilot maintenance' {
    BeforeEach {
        Initialize-TestRepository
    }
    It 'allows documentation changes and passes the write and deny rules' {
        $env:FAKE_COPILOT_MODE = 'docs'
        $result = Invoke-Script Invoke-CopilotMaintenance.ps1 -Task 'Document the setup.'
        $result.ExitCode | Should -Be 0 -Because $result.Output
        $result.Output | Should -Match 'Copilot changed: docs/fake.md'
        $call = (Get-AgentCall -Agent copilot)[0]
        $call | Should -Match '--allow-tool write\(docs/\*\*\)'
        $call | Should -Match '--deny-tool shell\(git push\)'
        $call | Should -Match '--no-ask-user'
        $call | Should -Not -Match 'Document the setup'
    }
    It 'fails when Copilot changes a file outside its scope' {
        $env:FAKE_COPILOT_MODE = 'outside'
        $result = Invoke-Script Invoke-CopilotMaintenance.ps1 -Task 'Anything'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'outside its scope: main.txt'
    }
    It 'fails when Copilot commits' {
        $env:FAKE_COPILOT_MODE = 'commit'
        $result = Invoke-Script Invoke-CopilotMaintenance.ps1 -Task 'Anything'
        $result.ExitCode | Should -Not -Be 0
        $result.Output | Should -Match 'moved HEAD'
    }
}

Describe 'Environment check' {
    BeforeEach {
        Initialize-TestRepository
    }
    It 'reports the hooks and the fake agents' {
        $result = Invoke-Script Test-AgentEnvironment.ps1
        $result.ExitCode | Should -Be 0 -Because $result.Output
        $result.Output | Should -Match 'codex fake 1.0.0'
    }
    It 'fails when the hooks are not installed' {
        $null = Invoke-Git config --unset core.hooksPath
        (Invoke-Script Test-AgentEnvironment.ps1).ExitCode | Should -Be 1
    }
}
