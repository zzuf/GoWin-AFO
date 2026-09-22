$ErrorActionPreference = 'Stop'

$setup = Join-Path $PSScriptRoot 'setup-msvc.cmd'

function Invoke-Setup([string]$arguments) {
    $output = & cmd.exe /d /c "call `"$setup`" $arguments && set VSCMD_ARG_TGT_ARCH" 2>&1
    return @{ ExitCode = $LASTEXITCODE; Output = ($output -join "`n") }
}

foreach ($case in @(
    @{ Arguments = 'amd64 10.0.26100.0'; Target = 'x64' },
    @{ Arguments = 'amd64_x86 10.0.26100.0'; Target = 'x86' }
)) {
    $result = Invoke-Setup $case.Arguments
    if ($result.ExitCode -ne 0 -or $result.Output -notmatch "(?m)^VSCMD_ARG_TGT_ARCH=$($case.Target)$") {
        throw "Setup failed for $($case.Arguments): exit $($result.ExitCode); $($result.Output)"
    }
}

$result = Invoke-Setup 'amd64 10.0.0.0'
if ($result.ExitCode -eq 0) {
    throw "Setup accepted an unavailable SDK: $($result.Output)"
}

$originalProgramFilesX86 = ${env:ProgramFiles(x86)}
try {
    ${env:ProgramFiles(x86)} = Join-Path $env:TEMP ('missing-vswhere-' + [guid]::NewGuid().ToString('N'))
    $result = Invoke-Setup 'amd64 10.0.26100.0'
    if ($result.ExitCode -eq 0) {
        throw "Setup succeeded without vswhere: $($result.Output)"
    }
} finally {
    ${env:ProgramFiles(x86)} = $originalProgramFilesX86
}

Write-Output 'setup-msvc tests passed'
exit 0
