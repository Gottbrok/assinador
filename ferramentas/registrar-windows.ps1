<#
Registra (ou remove) o Assinador de DESENVOLVIMENTO no Windows do usuário, para o teste manual antes
de existir o MSI (F6b): copia o programa para uma pasta do usuário, gera os manifestos de native
messaging pelo `manifestos.exe` (os mesmos IDs de extensão que o programa aceita: nunca manifesto
escrito à mão) e grava as chaves HKCU que o Chrome, o Edge, o Chromium e o Firefox leem. Não pede
administrador, e `-Remover` desfaz tudo.

Os dois executáveis são os do build `dev` (o artefato `assinador-dev-windows-*` do CI). O Windows
marca o que veio da internet e, por padrão, não roda script: desbloqueie os três arquivos baixados
e rode o script com a política só para esta execução:

  Unblock-File .\assinador-dev.exe, .\manifestos-dev.exe, .\registrar-windows.ps1
  powershell -ExecutionPolicy Bypass -File .\registrar-windows.ps1 -Programa .\assinador-dev.exe -Manifestos .\manifestos-dev.exe
  powershell -ExecutionPolicy Bypass -File .\registrar-windows.ps1 -Remover
#>
param(
  [string]$Programa,
  [string]$Manifestos,
  [switch]$Remover
)
$ErrorActionPreference = 'Stop'

$Nome = 'br.com.confidata.assinador'
$Pasta = Join-Path $env:LOCALAPPDATA 'ConfidataAssinadorDev'
$ChavesChromium = @(
  "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$Nome",
  "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\$Nome",
  "HKCU:\Software\Chromium\NativeMessagingHosts\$Nome"
)
$ChaveFirefox = "HKCU:\Software\Mozilla\NativeMessagingHosts\$Nome"

# Sobe apagando as chaves que ficaram VAZIAS (as que o registro criou para um navegador que não está
# instalado), até HKCU:\Software, sem nunca apagar uma que tenha outra subchave ou valor.
function Remover-ChavesVazias([string]$chave) {
  while ($chave -and $chave -ne 'HKCU:\Software' -and (Test-Path -LiteralPath $chave)) {
    $item = Get-Item -LiteralPath $chave
    if ($item.SubKeyCount -gt 0 -or $item.ValueCount -gt 0) { break }
    Remove-Item -LiteralPath $chave
    $chave = Split-Path -Path $chave -Parent
  }
}

if ($Remover) {
  foreach ($chave in $ChavesChromium + $ChaveFirefox) {
    if (Test-Path -LiteralPath $chave) { Remove-Item -LiteralPath $chave -Recurse }
    Remover-ChavesVazias (Split-Path -Path $chave -Parent)
  }
  if (Test-Path -LiteralPath $Pasta) { Remove-Item -LiteralPath $Pasta -Recurse }
  Write-Output 'Assinador de desenvolvimento removido.'
  exit 0
}

if (-not $Programa -or -not $Manifestos) {
  throw 'Informe -Programa e -Manifestos (ou -Remover).'
}
New-Item -ItemType Directory -Force -Path $Pasta | Out-Null
$Instalado = Join-Path $Pasta 'assinador.exe'
# Caminhos LITERAIS: sem o -LiteralPath, um `[` ou `]` no nome da pasta vira curinga.
Copy-Item -LiteralPath (Resolve-Path -LiteralPath $Programa).Path -Destination $Instalado -Force
# A cópia leva a marca de "veio da internet" do original; sem ela, o navegador lança o programa sem
# o aviso do SmartScreen no meio do native messaging.
Unblock-File -LiteralPath $Instalado
& (Resolve-Path -LiteralPath $Manifestos).Path -saida $Pasta -programa $Instalado
if ($LASTEXITCODE -ne 0) { throw "O manifestos.exe saiu com $LASTEXITCODE." }

foreach ($chave in $ChavesChromium) {
  New-Item -Path $chave -Force | Out-Null
  Set-Item -Path $chave -Value (Join-Path $Pasta 'chromium.json')
}
New-Item -Path $ChaveFirefox -Force | Out-Null
Set-Item -Path $ChaveFirefox -Value (Join-Path $Pasta 'firefox.json')
Write-Output "Assinador de desenvolvimento registrado em $Pasta."
