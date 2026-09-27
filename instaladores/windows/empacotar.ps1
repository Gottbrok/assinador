<#
Monta e valida os MSI de DESENVOLVIMENTO do Assinador para o Windows (F6b): o por usuário (sem
administrador) e o por máquina (GPO e Intune), da arquitetura pedida. O programa leva a tag `dev` e os
manifestos saem de `nativo/cmd/manifestos -relativo`, com as mesmas extensões que o programa aceita
(regra 13 do CLAUDE.md). É o par dos `.deb` e `.rpm` de `instaladores/linux` (regra 14): o MSI de
produção, com os IDs das lojas e assinado, é da F7a. Sem assinatura até o certificado de assinatura de
código (F6b-ii).

Roda no Windows, no PowerShell 7 (`pwsh`), com o Go do `nativo/go.mod` e o WiX 5.0.2 (`dotnet tool
install --global wix --version 5.0.2`; o 6 exige o EULA da taxa de manutenção da OSMF):

  .\instaladores\windows\empacotar.ps1 -VersaoDoMsi 0.1.1 -VersaoDoPrograma 1.0.0 -Saida dist -Arquitetura amd64

São DUAS versões, como no Linux (`1.0.0~dev.1` é o pacote, e o programa informa `1.0.0`):
- `-VersaoDoMsi` é a do pacote, a que o Windows Installer compara para atualizar: `X.Y.Z` até
  `255.255.65535`, abaixo da primeira publicação (o MSI de produção atualiza o de desenvolvimento);
- `-VersaoDoPrograma` é a que o programa informa (`main.versao`), a da PRÓXIMA publicação, que a
  biblioteca compara com a versão mínima (`protocolo/PROTOCOLO.md`).

Escreve `assinador-dev-windows-<arquitetura>-usuario.msi` e `-maquina.msi` na pasta de saída, e os
valida pelos ICE (`wix msi validate`), que o `wix build` do WiX 5 não roda. No PowerShell 7: o 5.1 lê
o script sem BOM como ANSI.
#>
#Requires -Version 7.2
param(
  [Parameter(Mandatory)][string]$VersaoDoMsi,
  [Parameter(Mandatory)][string]$VersaoDoPrograma,
  [Parameter(Mandatory)][string]$Saida,
  [Parameter(Mandatory)][ValidateSet('amd64', 'arm64')][string]$Arquitetura
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# X.Y.Z sem zero à esquerda (a biblioteca compara número a número, e o Windows Installer também).
$FormaDaVersao = '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$'
if ($VersaoDoMsi -notmatch $FormaDaVersao -or [long]$Matches[1] -gt 255 -or [long]$Matches[2] -gt 255 -or [long]$Matches[3] -gt 65535) {
  throw "A versão do MSI $VersaoDoMsi não é X.Y.Z nos limites do Windows Installer (até 255.255.65535, sem zero à esquerda)."
}
if ($VersaoDoPrograma -notmatch $FormaDaVersao) {
  throw "A versão do programa $VersaoDoPrograma não é X.Y.Z (sem zero à esquerda)."
}

$Raiz = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
New-Item -ItemType Directory -Force -Path $Saida | Out-Null
$SaidaAbsoluta = (Resolve-Path -LiteralPath $Saida).Path
$Trabalho = Join-Path ([System.IO.Path]::GetTempPath()) ('assinador-msi-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Trabalho | Out-Null

# As variáveis do Go que o script troca, com o valor que a sessão de quem o chamou tinha: voltam no fim.
$AmbienteAnterior = @{}
foreach ($nome in 'GOOS', 'GOARCH', 'CGO_ENABLED') { $AmbienteAnterior[$nome] = [Environment]::GetEnvironmentVariable($nome) }
# Valor `$null` apaga a variável (o `[Environment]::SetEnvironmentVariable` receberia texto vazio: o
# PowerShell converte `$null` em "" nos parâmetros `string` do .NET).
function Definir-Ambiente([hashtable]$valores) {
  foreach ($nome in $valores.Keys) {
    if ($null -eq $valores[$nome]) { Remove-Item -LiteralPath "Env:$nome" -ErrorAction SilentlyContinue }
    else { Set-Item -LiteralPath "Env:$nome" -Value $valores[$nome] }
  }
}

try {
  Push-Location -LiteralPath (Join-Path $Raiz 'nativo')
  try {
    # O programa, compilado para a arquitetura do pacote (sem cgo: o provedor do Windows é por syscall).
    Definir-Ambiente @{ GOOS = 'windows'; GOARCH = $Arquitetura; CGO_ENABLED = '0' }
    & go build -trimpath -tags dev -ldflags "-X main.versao=$VersaoDoPrograma" -o (Join-Path $Trabalho 'assinador.exe') ./cmd/assinador
    if ($LASTEXITCODE -ne 0) { throw "O go build saiu com $LASTEXITCODE." }
    # O gerador roda na máquina que empacota, e escreve os manifestos apontando o programa pelo nome.
    Definir-Ambiente @{ GOOS = $null; GOARCH = $null; CGO_ENABLED = $null }
    & go run -tags dev ./cmd/manifestos -saida $Trabalho -programa assinador.exe -relativo
    if ($LASTEXITCODE -ne 0) { throw "O gerador de manifestos saiu com $LASTEXITCODE." }
  } finally {
    Pop-Location
    Definir-Ambiente $AmbienteAnterior
  }

  $ArquiteturaDoWix = @{ amd64 = 'x64'; arm64 = 'arm64' }[$Arquitetura]
  $Fonte = Join-Path $Raiz 'instaladores\windows\Assinador.wxs'
  foreach ($escopo in 'usuario', 'maquina') {
    $msi = Join-Path $SaidaAbsoluta "assinador-dev-windows-$Arquitetura-$escopo.msi"
    & wix build -arch $ArquiteturaDoWix -d "Versao=$VersaoDoMsi" -d "Escopo=$escopo" -d "Origem=$Trabalho" -o $msi $Fonte
    if ($LASTEXITCODE -ne 0) { throw "O wix build ($escopo) saiu com $LASTEXITCODE." }
    # Os ICE, com o .wixpdb ao lado para a linha do fonte. No MSI por usuário o ICE91 avisa dos arquivos
    # no perfil, e a Microsoft o dá por inofensivo em pacote só por usuário: aviso não reprova.
    & wix msi validate -intermediateFolder (Join-Path $Trabalho "ice-$escopo") $msi
    if ($LASTEXITCODE -ne 0) { throw "O wix msi validate ($escopo) saiu com $LASTEXITCODE." }
  }
  # O .wixpdb (símbolos do instalador) não vai para o artefato.
  Get-ChildItem -LiteralPath $SaidaAbsoluta -Filter '*.wixpdb' | Remove-Item
} finally {
  Remove-Item -LiteralPath $Trabalho -Recurse -Force -ErrorAction SilentlyContinue
}
Get-ChildItem -LiteralPath $SaidaAbsoluta -Filter 'assinador-dev-windows-*.msi' | Select-Object Name, Length | Format-Table -AutoSize
