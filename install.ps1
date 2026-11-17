param(
  [Parameter(Mandatory = $true)][string]$Source
)

$ErrorActionPreference = 'Stop'

$destDir = Join-Path $env:LOCALAPPDATA 'Programs\QuickMD'
New-Item -ItemType Directory -Force -Path $destDir | Out-Null
$dest = Join-Path $destDir 'quickmd.exe'
Copy-Item -LiteralPath $Source -Destination $dest -Force
Unblock-File -LiteralPath $dest

$classes = 'HKCU:\Software\Classes'
$progId = 'QuickMD.md'
$cmd = '"' + $dest + '" "%1"'

New-Item -Force -Path "$classes\$progId\shell\open\command" | Out-Null
Set-ItemProperty -Path "$classes\$progId" -Name '(Default)' -Value 'Markdown document'
New-Item -Force -Path "$classes\$progId\DefaultIcon" | Out-Null
Set-ItemProperty -Path "$classes\$progId\DefaultIcon" -Name '(Default)' -Value ('"' + $dest + '",0')
Set-ItemProperty -Path "$classes\$progId\shell\open\command" -Name '(Default)' -Value $cmd

foreach ($ext in '.md', '.markdown', '.mdx') {
  New-Item -Force -Path "$classes\$ext\OpenWithProgids" | Out-Null
  New-ItemProperty -Path "$classes\$ext\OpenWithProgids" -Name $progId -PropertyType String -Value '' -Force | Out-Null
}

$app = "$classes\Applications\quickmd.exe"
New-Item -Force -Path "$app\shell\open\command" | Out-Null
Set-ItemProperty -Path $app -Name 'FriendlyAppName' -Value 'QuickMD'
Set-ItemProperty -Path "$app\shell\open\command" -Name '(Default)' -Value $cmd
foreach ($ext in '.md', '.markdown', '.mdx') {
  New-Item -Force -Path "$app\SupportedTypes" | Out-Null
  New-ItemProperty -Path "$app\SupportedTypes" -Name $ext -PropertyType String -Value '' -Force | Out-Null
}

$cap = 'HKCU:\Software\QuickMD\Capabilities'
New-Item -Force -Path "$cap\FileAssociations" | Out-Null
Set-ItemProperty -Path $cap -Name 'ApplicationName' -Value 'QuickMD'
Set-ItemProperty -Path $cap -Name 'ApplicationDescription' -Value 'Fast native Markdown editor'
foreach ($ext in '.md', '.markdown', '.mdx') {
  Set-ItemProperty -Path "$cap\FileAssociations" -Name $ext -Value $progId
}
New-Item -Force -Path 'HKCU:\Software\RegisteredApplications' | Out-Null
Set-ItemProperty -Path 'HKCU:\Software\RegisteredApplications' -Name 'QuickMD' -Value 'Software\QuickMD\Capabilities'

Write-Output "installed: $dest"
Write-Output ("command: " + (Get-ItemProperty "$classes\$progId\shell\open\command").'(default)')

$userChoice = $null
try {
  $userChoice = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.md\UserChoice' -ErrorAction Stop).ProgId
} catch { }
Write-Output "current .md user choice: $userChoice"

Start-Process 'ms-settings:defaultapps?registeredAppUser=QuickMD'
Write-Output 'opened Windows Default Apps for QuickMD'
