<#
Monta os MSI de DESENVOLVIMENTO do Assinador para o Windows (F6b): o por usuário (sem administrador) e o
por máquina (GPO e Intune), da arquitetura pedida. O programa leva a tag `dev` e os manifestos saem de
`nativo/cmd/manifestos -relativo`, com as mesmas extensões que o programa aceita (regra 13 do CLAUDE.md).
É o par dos `.deb` e `.rpm` de `instaladores/linux` (regra 14): o MSI de produção, com os IDs das lojas e
assinado, é da F7a. Sem assinatura até o certificado de assinatura de código (F6b-ii).

Roda no Windows, com o Go do `nativo/go.mod` e o WiX 5.0.2 (`dotnet tool install --global wix --version
5.0.2`; o 6 exige o EULA da taxa de manutenção da OSMF):

  .\instaladores\windows\empacotar.ps1 -Versao 0.1.1 -Saida dist -Arquitetura amd64

Escreve `assinador-dev-windows-<arquitetura>-usuario.msi` e `-maquina.msi` na pasta de saída. No
PowerShell 7 (`pwsh`), como o `testar-instalador.ps1`: o 5.1 lê o script sem BOM como ANSI.
#>
#Requires -Version 7.2
param(
  [Parameter(Mandatory)][string]$Versao,
  [Parameter(Mandatory)][string]$Saida,
  [Parameter(Mandatory)][ValidateSet('amd64', 'arm64')][string]$Arquitetura
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# A versão do MSI é X.Y.Z nos limites do Windows Installer (255.255.65535; o quarto campo ele ignora na
# atualização), e é também a que o programa informa (`main.versao`), que a biblioteca compara com a
# versão mínima em X.Y.Z.
if ($Versao -notmatch '^(\d{1,3})\.(\d{1,3})\.(\d{1,5})$' -or [int]$Matches[1] -gt 255 -or [int]$Matches[2] -gt 255 -or [int]$Matches[3] -gt 65535) {
  throw "A versão $Versao não é X.Y.Z nos limites do Windows Installer (até 255.255.65535)."
}

$Raiz = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
New-Item -ItemType Directory -Force -Path $Saida | Out-Null
$SaidaAbsoluta = (Resolve-Path -LiteralPath $Saida).Path
$Trabalho = Join-Path ([System.IO.Path]::GetTempPath()) ('assinador-msi-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Trabalho | Out-Null
try {
  Push-Location -LiteralPath (Join-Path $Raiz 'nativo')
  try {
    # O programa, compilado para a arquitetura do pacote (sem cgo: o provedor do Windows é por syscall).
    $env:GOOS = 'windows'
    $env:GOARCH = $Arquitetura
    $env:CGO_ENABLED = '0'
    & go build -trimpath -tags dev -ldflags "-X main.versao=$Versao" -o (Join-Path $Trabalho 'assinador.exe') ./cmd/assinador
    if ($LASTEXITCODE -ne 0) { throw "O go build saiu com $LASTEXITCODE." }
    # O gerador roda na máquina que empacota, e escreve os manifestos apontando o programa pelo nome.
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
    & go run -tags dev ./cmd/manifestos -saida $Trabalho -programa assinador.exe -relativo
    if ($LASTEXITCODE -ne 0) { throw "O gerador de manifestos saiu com $LASTEXITCODE." }
  } finally {
    Pop-Location
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
  }

  $ArquiteturaDoWix = @{ amd64 = 'x64'; arm64 = 'arm64' }[$Arquitetura]
  $Fonte = Join-Path $Raiz 'instaladores\windows\Assinador.wxs'
  foreach ($escopo in 'usuario', 'maquina') {
    $msi = Join-Path $SaidaAbsoluta "assinador-dev-windows-$Arquitetura-$escopo.msi"
    & wix build -arch $ArquiteturaDoWix -d "Versao=$Versao" -d "Escopo=$escopo" -d "Origem=$Trabalho" -o $msi $Fonte
    if ($LASTEXITCODE -ne 0) { throw "O wix build ($escopo) saiu com $LASTEXITCODE." }
  }
  # O .wixpdb (símbolos do instalador) não vai para o artefato.
  Get-ChildItem -LiteralPath $SaidaAbsoluta -Filter '*.wixpdb' | Remove-Item
} finally {
  Remove-Item -LiteralPath $Trabalho -Recurse -Force -ErrorAction SilentlyContinue
}
Get-ChildItem -LiteralPath $SaidaAbsoluta -Filter 'assinador-dev-windows-*.msi' | Select-Object Name, Length | Format-Table -AutoSize
