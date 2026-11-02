# Smoke test of the Windows installer on a real Windows user session:
# silent install, `webmux-tray quit`, upgrade over a running tray, refusal
# beside a foreign tray, wrong-architecture refusal and uninstall.
#
# It installs into the current user's profile and leaves tray.json/tray.log
# behind by design; run it on a CI runner or a throwaway account.
param(
  [Parameter(Mandatory)] [string] $Setup,          # installer for this machine
  [Parameter(Mandatory)] [string] $SetupOtherArch, # installer for the other arch
  [Parameter(Mandatory)] [string] $ServerSha256,   # SHA-256 of the bundled server
  [Parameter(Mandatory)] [string] $Version,
  [Parameter(Mandatory)] [string] $Arch
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\WebMux'
$tray = Join-Path $installDir 'webmux-tray.exe'
$uninstaller = Join-Path $installDir 'uninstall.exe'
$shortcut = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\WebMux.lnk'
$uninstKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\WebMux'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$preseed = Join-Path $env:LOCALAPPDATA "WebMux\downloads\webmux-$Version-linux-$Arch"
$work = Join-Path ([IO.Path]::GetTempPath()) ('webmux-smoke-' + [guid]::NewGuid())
New-Item -ItemType Directory $work | Out-Null

function Check([bool] $ok, [string] $what) {
  if (-not $ok) { throw "FAILED: $what" }
  Write-Host "ok - $what"
}

# Waits for the process only: Windows PowerShell's Start-Process -Wait also
# waits for descendants, such as the tray the installer starts. Arguments
# are joined unquoted by Start-Process, so quote any with spaces.
function Run([string] $exe, [string[]] $argv) {
  $quoted = @($argv | ForEach-Object { if ($_ -match '\s') { "`"$_`"" } else { $_ } })
  $p = Start-Process -FilePath $exe -ArgumentList $quoted -PassThru -WindowStyle Hidden
  $null = $p.Handle # keeps the exit code readable after exit
  $p.WaitForExit()
  return $p.ExitCode
}

# Always an array (the leading comma stops PowerShell from unrolling it).
function TrayProcesses([string] $path) {
  , @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
      try { $_.Path -and ($_.Path -ieq $path) } catch { $false }
    })
}

function WaitFor([scriptblock] $cond, [int] $seconds = 15) {
  $deadline = (Get-Date).AddSeconds($seconds)
  while ((Get-Date) -lt $deadline) {
    if (& $cond) { return $true }
    Start-Sleep -Milliseconds 200
  }
  return [bool](& $cond)
}

function RunValue {
  (Get-ItemProperty $runKey -ErrorAction SilentlyContinue).WebMuxTray
}

# A stand-in for an older tray: it holds the instance mutex but has no quit
# event, so it never answers `quit`.
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
Add-Type -OutputType WindowsApplication -OutputAssembly (Join-Path $work 'fake.exe') -TypeDefinition @'
using System.Threading;
static class FakeTray {
  static void Main(string[] args) {
    var m = new Mutex(false, args[0]);
    Thread.Sleep(Timeout.Infinite);
    System.GC.KeepAlive(m);
  }
}
'@
$fakeDir = Join-Path $work 'other'
New-Item -ItemType Directory $fakeDir | Out-Null
$fake = Join-Path $fakeDir 'webmux-tray.exe'
Copy-Item (Join-Path $work 'fake.exe') $fake
function MutexExists {
  $m = $null
  if ([Threading.Mutex]::TryOpenExisting("Local\WebMuxTray-$sid", [ref] $m)) { $m.Dispose(); return $true }
  return $false
}
function StartFake {
  $p = Start-Process -FilePath $fake -ArgumentList "Local\WebMuxTray-$sid" -PassThru
  if (-not (WaitFor { MutexExists } 5)) { throw 'fake tray did not create the instance mutex' }
  return $p
}

try {
  # Wrong architecture: refused before anything is written.
  Check ((Run $SetupOtherArch @('/S')) -eq 2) 'other-architecture installer exits 2'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $uninstKey)) 'other-architecture installer writes nothing'

  # A foreign tray that cannot be closed: refused before anything is written.
  $foreign = StartFake
  Check ((Run $Setup @('/S')) -eq 3) 'installer exits 3 beside a foreign unresponsive tray'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $uninstKey)) 'refused installer writes nothing'
  Check (-not $foreign.HasExited) 'foreign tray left running'
  Stop-Process -Id $foreign.Id

  # Fresh silent install.
  Check ((Run $Setup @('/S')) -eq 0) 'silent install exits 0'
  Check ((Test-Path $tray) -and (Test-Path $uninstaller)) 'tray and uninstaller installed'
  Check (Test-Path $shortcut) 'Start menu shortcut created'
  $u = Get-ItemProperty $uninstKey
  Check ($u.DisplayName -eq 'WebMux' -and $u.DisplayVersion -eq $Version) 'uninstall entry name and version'
  Check ($u.InstallLocation -ieq $installDir -and $u.QuietUninstallString -like '*uninstall.exe" /S') 'uninstall entry paths'
  Check ((RunValue) -ieq "`"$tray`"") 'login item points to the installed tray'
  Check ((Get-FileHash $preseed -Algorithm SHA256).Hash -ieq $ServerSha256) 'bundled server pre-seeded in the tray download cache'
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'installer started the tray'

  # quit: the installed tray exits; with no tray, quit returns at once.
  Check ((Run $tray @('quit')) -eq 0) 'quit exits 0 when the tray exits'
  Check ((TrayProcesses $tray).Count -eq 0) 'tray process gone after quit'
  $t = Measure-Command { $code = Run $tray @('quit') }
  Check ($code -eq 0 -and $t.TotalSeconds -lt 5) 'quit without a tray exits 0 at once'
  Check ((Run $tray @('quit', '--bogus')) -eq 2) 'quit rejects unknown arguments'

  # quit and an unresponsive tray: only the --kill-path one is ended.
  $old = StartFake
  Check ((Run $tray @('quit', '--kill-path', $tray)) -eq 1) 'quit exits 1 when an unresponsive tray runs elsewhere'
  Check (-not $old.HasExited) 'unresponsive tray at another path survives'
  Check ((Run $tray @('quit', '--kill-path', $fake)) -eq 0) 'quit --kill-path ends the unresponsive tray at that path'
  Check (WaitFor { $old.HasExited } 5) 'unresponsive tray terminated'

  # Upgrade over a running tray.
  Start-Process -FilePath $tray | Out-Null
  Check (WaitFor { (TrayProcesses $tray).Count -eq 1 }) 'tray running before upgrade'
  $before = (TrayProcesses $tray)[0].Id
  Check ((Run $Setup @('/S')) -eq 0) 'upgrade over a running tray exits 0'
  Check (WaitFor { $p = TrayProcesses $tray; $p.Count -eq 1 -and $p[0].Id -ne $before }) 'old tray replaced by a new one'

  # Uninstall keeps settings, logs and a login item that points elsewhere.
  $settings = Join-Path $env:APPDATA 'WebMux\tray.json'
  if (-not (Test-Path $settings)) { New-Item -ItemType File -Force $settings | Out-Null }
  Set-ItemProperty $runKey WebMuxTray "`"$fake`""
  Check ((Run $uninstaller @('/S')) -eq 0) 'silent uninstall exits 0'
  Check (WaitFor { -not (Test-Path $uninstKey) }) 'uninstall entry removed'
  Check ((TrayProcesses $tray).Count -eq 0) 'tray closed by the uninstaller'
  Check (-not (Test-Path $installDir) -and -not (Test-Path $shortcut)) 'install directory and shortcut removed'
  Check (-not (Test-Path $preseed)) 'pre-seeded server removed'
  Check ((Test-Path $settings) -and (Test-Path (Join-Path $env:LOCALAPPDATA 'WebMux\tray.log'))) 'tray settings and log kept'
  Check ((RunValue) -ieq "`"$fake`"") 'login item pointing elsewhere kept'
  Remove-ItemProperty $runKey WebMuxTray
} finally {
  Get-Process -ErrorAction SilentlyContinue | Where-Object {
    try { $_.Path -and $_.Path.StartsWith($work, [StringComparison]::OrdinalIgnoreCase) } catch { $false }
  } | Stop-Process -Force -ErrorAction SilentlyContinue
  Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
Write-Host 'windows installer smoke: all checks passed'
