<#
Registra (ou remove) o Assinador de DESENVOLVIMENTO no Windows do usuário, para o teste manual antes
de existir o MSI (F6b): copia o programa para uma pasta do usuário, gera os manifestos de native
messaging pelo `manifestos.exe` (os mesmos IDs de extensão que o programa aceita: nunca manifesto
escrito à mão) e grava as chaves HKCU que o Chrome, o Edge, o Chromium e o Firefox leem. Não pede
administrador, e `-Remover` desfaz tudo.

Os dois executáveis são os do build `dev` (o artefato `assinador-dev-windows-*` do CI):

  .\registrar-windows.ps1 -Programa .\assinador-dev.exe -Manifestos .\manifestos-dev.exe
  .\registrar-windows.ps1 -Remover
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

if ($Remover) {
  foreach ($chave in $ChavesChromium + $ChaveFirefox) {
    if (Test-Path $chave) { Remove-Item -Path $chave -Recurse }
  }
  if (Test-Path $Pasta) { Remove-Item -Path $Pasta -Recurse }
  Write-Output 'Assinador de desenvolvimento removido.'
  exit 0
}

if (-not $Programa -or -not $Manifestos) {
  throw 'Informe -Programa e -Manifestos (ou -Remover).'
}
New-Item -ItemType Directory -Force -Path $Pasta | Out-Null
$Instalado = Join-Path $Pasta 'assinador.exe'
Copy-Item -Path (Resolve-Path $Programa).Path -Destination $Instalado -Force
& (Resolve-Path $Manifestos).Path -saida $Pasta -programa $Instalado
if ($LASTEXITCODE -ne 0) { throw "O manifestos.exe saiu com $LASTEXITCODE." }

foreach ($chave in $ChavesChromium) {
  New-Item -Path $chave -Force | Out-Null
  Set-Item -Path $chave -Value (Join-Path $Pasta 'chromium.json')
}
New-Item -Path $ChaveFirefox -Force | Out-Null
Set-Item -Path $ChaveFirefox -Value (Join-Path $Pasta 'firefox.json')
Write-Output "Assinador de desenvolvimento registrado em $Pasta."
