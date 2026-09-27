<#
Prova um MSI de DESENVOLVIMENTO do Assinador no Windows (F6b), sem cartão: o par do
`instaladores/linux/testar-pacotes.sh`. Fotografa as chaves de native messaging dos navegadores e as
pastas; (com -MsiAnterior) instala a versão anterior e prova a ATUALIZAÇÃO por cima dela; confere o
programa, os manifestos e as chaves; conversa com o programa instalado pelo MESMO caminho que o
navegador segue (a chave aponta o manifesto, o manifesto aponta o programa pelo nome, na pasta dele);
desinstala; e reprova se o que ficou não for o que havia antes.

O MSI por usuário também prova, pela estrutura, que não pede administrador (o bit de "privilégio
elevado não exigido" do resumo do pacote, e o ALLUSERS vazio). O por máquina precisa de administrador,
que o executor do CI tem.

  .\instaladores\windows\testar-instalador.ps1 -Msi dist\assinador-dev-windows-amd64-usuario.msi -Escopo usuario -Versao 0.1.2 -MsiAnterior anterior\assinador-dev-windows-amd64-usuario.msi -VersaoAnterior 0.0.1

No PowerShell 7 (`pwsh`): o 5.1 não tem o `ProcessStartInfo.ArgumentList` e lê o script sem BOM como
ANSI.
#>
#Requires -Version 7.2
param(
  [Parameter(Mandatory)][string]$Msi,
  [Parameter(Mandatory)][ValidateSet('usuario', 'maquina')][string]$Escopo,
  [Parameter(Mandatory)][string]$Versao,
  [string]$MsiAnterior,
  [string]$VersaoAnterior
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ([bool]$MsiAnterior -ne [bool]$VersaoAnterior) { throw 'Informe -MsiAnterior e -VersaoAnterior juntos.' }
$Raiz = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$Msi = (Resolve-Path -LiteralPath $Msi).Path
if ($MsiAnterior) { $MsiAnterior = (Resolve-Path -LiteralPath $MsiAnterior).Path }
$Nome = 'br.com.confidata.assinador'
$IdDev = (Get-Content -LiteralPath (Join-Path $Raiz 'protocolo\extensao-dev.json') -Raw | ConvertFrom-Json).idChrome
if ($Escopo -eq 'usuario') {
  $Pasta = Join-Path $env:LOCALAPPDATA 'Programs\Confidata Assinador'
  $Colmeia = 'HKCU:'
  $NomeDaColmeia = 'HKEY_CURRENT_USER'
} else {
  $Pasta = Join-Path $env:ProgramFiles 'Confidata Assinador'
  $Colmeia = 'HKLM:'
  $NomeDaColmeia = 'HKEY_LOCAL_MACHINE'
}
# A chave de cada navegador e o manifesto para o qual ela aponta.
$Folhas = [ordered]@{
  "Software\Google\Chrome\NativeMessagingHosts\$Nome"  = 'chromium.json'
  "Software\Microsoft\Edge\NativeMessagingHosts\$Nome" = 'chromium.json'
  "Software\Chromium\NativeMessagingHosts\$Nome"       = 'chromium.json'
  "Software\Mozilla\NativeMessagingHosts\$Nome"        = 'firefox.json'
}

function Falhar([string]$motivo) { throw "[testar-instalador $Escopo] $motivo" }

# As chaves-mãe de cada folha, de `Software\<fabricante>` até `NativeMessagingHosts`.
function Maes-DaFolha([string]$folha) {
  $partes = $folha.Split('\')
  for ($i = 2; $i -lt $partes.Length; $i++) { ($partes[0..($i - 1)] -join '\') }
}

# A foto do que o navegador lê: a existência de cada chave-mãe e tudo o que há sob cada
# `NativeMessagingHosts` (chaves e valores, de qualquer programa). Fotografar a árvore inteira do
# fabricante pegaria o que o próprio navegador ou o atualizador dele escrevem durante o teste.
function Foto-DoRegistro {
  $linhas = [System.Collections.Generic.List[string]]::new()
  $maes = $Folhas.Keys | ForEach-Object { Maes-DaFolha $_ } | Sort-Object -Unique
  foreach ($mae in $maes) {
    $caminho = "$Colmeia\$mae"
    if (-not (Test-Path -LiteralPath $caminho)) { continue }
    $linhas.Add("existe $NomeDaColmeia\$mae")
    if ($mae -like '*\NativeMessagingHosts') {
      $chaves = @(Get-Item -LiteralPath $caminho) + @(Get-ChildItem -LiteralPath $caminho -Recurse -ErrorAction SilentlyContinue)
      foreach ($c in $chaves) {
        $linhas.Add("chave $($c.Name)")
        foreach ($v in $c.GetValueNames()) { $linhas.Add("valor $($c.Name) [$v] = $($c.GetValue($v))") }
      }
    }
  }
  # Num Windows limpo nenhum navegador tem a chave, e a foto é vazia: sempre um conjunto.
  , [System.Collections.Generic.HashSet[string]]::new([string[]]$linhas.ToArray(), [System.StringComparer]::OrdinalIgnoreCase)
}

function Chamar-Msiexec([string[]]$argumentos, [string]$rotulo) {
  $log = Join-Path ([System.IO.Path]::GetTempPath()) ("assinador-msiexec-$rotulo-" + [guid]::NewGuid() + '.log')
  $p = Start-Process -FilePath 'msiexec.exe' -ArgumentList ($argumentos + @('/qn', '/norestart', '/l*v', "`"$log`"")) -Wait -PassThru
  if ($p.ExitCode -notin 0, 3010) {
    Get-Content -LiteralPath $log -Tail 80 -ErrorAction SilentlyContinue | Write-Output
    Falhar "o msiexec ($rotulo) saiu com $($p.ExitCode)"
  }
}

# O WindowsInstaller.Installer é COM sem biblioteca de tipos para o PowerShell: tudo por InvokeMember.
$Instalador = New-Object -ComObject WindowsInstaller.Installer
function Chamar([object]$objeto, [string]$membro, [System.Reflection.BindingFlags]$tipo, [object[]]$argumentos) {
  $objeto.GetType().InvokeMember($membro, $tipo, $null, $objeto, $argumentos)
}
function Propriedade-DoMsi([string]$caminho, [string]$propriedade) {
  $banco = Chamar $Instalador 'OpenDatabase' InvokeMethod @($caminho, 0)
  $consulta = Chamar $banco 'OpenView' InvokeMethod @("SELECT ``Value`` FROM ``Property`` WHERE ``Property`` = '$propriedade'")
  Chamar $consulta 'Execute' InvokeMethod $null | Out-Null
  $registro = Chamar $consulta 'Fetch' InvokeMethod $null
  $valor = if ($registro) { Chamar $registro 'StringData' GetProperty @(1) } else { $null }
  Chamar $consulta 'Close' InvokeMethod $null | Out-Null
  # Solta o arquivo: o msiexec o abre depois.
  foreach ($o in $registro, $consulta, $banco) { if ($o) { [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($o) } }
  $valor
}
function Produtos-Relacionados([string]$upgradeCode) {
  $lista = Chamar $Instalador 'RelatedProducts' GetProperty @($upgradeCode)
  Chamar $lista 'Count' GetProperty $null
}

# Lê exatamente `n` bytes do fluxo (a leitura pode voltar com menos).
function Ler-Bytes([System.IO.Stream]$fluxo, [int]$n) {
  $buffer = [byte[]]::new($n)
  $lidos = 0
  while ($lidos -lt $n) {
    $r = $fluxo.Read($buffer, $lidos, $n - $lidos)
    if ($r -le 0) { Falhar "o programa fechou a saída depois de $lidos de $n bytes" }
    $lidos += $r
  }
  , $buffer
}

# Conversa com o programa como o Chrome conversa: lançado com a origem da extensão, um quadro de 4 bytes
# (o tamanho, na ordem da máquina) mais o JSON na entrada, e o mesmo na saída.
function Conversar-Ola([string]$programa) {
  $inicio = [System.Diagnostics.ProcessStartInfo]::new($programa)
  $inicio.ArgumentList.Add("chrome-extension://$IdDev/")
  $inicio.UseShellExecute = $false
  $inicio.RedirectStandardInput = $true
  $inicio.RedirectStandardOutput = $true
  $inicio.RedirectStandardError = $true
  $p = [System.Diagnostics.Process]::Start($inicio)
  # O erro padrão lido em paralelo: um aviso que encha o buffer dele não trava o programa.
  $erro = $p.StandardError.ReadToEndAsync()
  try {
    $corpo = [System.Text.Encoding]::UTF8.GetBytes('{"v":1,"id":"msi-ola","op":"ola","origem":"https://demo.confidata.app"}')
    $entrada = $p.StandardInput.BaseStream
    $entrada.Write([System.BitConverter]::GetBytes([uint32]$corpo.Length), 0, 4)
    $entrada.Write($corpo, 0, $corpo.Length)
    $entrada.Flush()
    $saida = $p.StandardOutput.BaseStream
    $tamanho = [System.BitConverter]::ToUInt32((Ler-Bytes $saida 4), 0)
    if ($tamanho -eq 0 -or $tamanho -gt 1MB) { Falhar "quadro de resposta com $tamanho bytes" }
    $resposta = [System.Text.Encoding]::UTF8.GetString((Ler-Bytes $saida ([int]$tamanho))) | ConvertFrom-Json
    # Entrada fechada: o programa sai sozinho (é assim que o navegador o encerra).
    $p.StandardInput.Close()
    if (-not $p.WaitForExit(15000)) { $p.Kill(); Falhar 'o programa não saiu depois de a entrada fechar' }
    $resposta
  } finally {
    if (-not $p.HasExited) { $p.Kill() }
    [void]$erro.Wait(5000)
  }
}

function Conferir-Instalado([string]$versaoEsperada) {
  if (-not (Test-Path -LiteralPath (Join-Path $Pasta 'assinador.exe'))) { Falhar "o programa não está em $Pasta" }
  foreach ($folha in $Folhas.Keys) {
    $chave = "$Colmeia\$folha"
    if (-not (Test-Path -LiteralPath $chave)) { Falhar "a chave $chave não foi criada" }
    $apontado = (Get-Item -LiteralPath $chave).GetValue('')
    $esperado = Join-Path $Pasta $Folhas[$folha]
    if ($apontado -ne $esperado) { Falhar "a chave $chave aponta para '$apontado', e não para '$esperado'" }
    # O caminho que o navegador segue: o manifesto, e o programa pelo nome, na pasta do manifesto.
    $manifesto = Get-Content -LiteralPath $apontado -Raw | ConvertFrom-Json
    if ($manifesto.name -ne $Nome -or $manifesto.type -ne 'stdio') { Falhar "manifesto $apontado fora da forma" }
    if ($manifesto.path -ne 'assinador.exe') { Falhar "o manifesto $apontado aponta '$($manifesto.path)', e não o programa pelo nome" }
    if ($Folhas[$folha] -eq 'firefox.json') {
      if (@($manifesto.allowed_extensions) -notcontains 'assinador@confidata.com.br') { Falhar 'o manifesto do Firefox não traz a extensão' }
    } elseif (@($manifesto.allowed_origins) -notcontains "chrome-extension://$IdDev/") {
      Falhar "o manifesto $apontado não traz a extensão de desenvolvimento"
    }
  }
  $programa = Join-Path (Split-Path -Parent (Join-Path $Pasta 'chromium.json')) (Get-Content -LiteralPath (Join-Path $Pasta 'chromium.json') -Raw | ConvertFrom-Json).path
  $informada = (& $programa versao | Out-String).Trim()
  if ($LASTEXITCODE -ne 0 -or $informada -ne $versaoEsperada) { Falhar "o programa instalado informa '$informada', e não $versaoEsperada" }
  $ola = Conversar-Ola $programa
  if (-not $ola.ok -or $ola.id -ne 'msi-ola' -or $ola.dados.versao -ne $versaoEsperada) { Falhar "o olá do programa instalado veio fora do esperado: $($ola | ConvertTo-Json -Compress -Depth 5)" }
  Write-Output "instalado: $programa informa $informada e responde ao olá (plataforma $($ola.dados.plataforma))"
}

# --- A estrutura do pacote ---------------------------------------------------------------------------
$UpgradeCode = Propriedade-DoMsi $Msi 'UpgradeCode'
if (-not $UpgradeCode) { Falhar 'o MSI não tem UpgradeCode' }
if ($Escopo -eq 'usuario') {
  $resumo = Chamar $Instalador 'SummaryInformation' GetProperty @($Msi, 0)
  $contagem = [int](Chamar $resumo 'Property' GetProperty @(15))
  [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($resumo)
  # PID_WORDCOUNT, bit 8: o pacote não exige privilégio elevado (o Windows não pede UAC).
  if (($contagem -band 8) -eq 0) { Falhar "o MSI por usuário exige privilégio elevado (Word Count $contagem)" }
  if ((Propriedade-DoMsi $Msi 'ALLUSERS') -in '1', '2') { Falhar 'o MSI por usuário declara ALLUSERS' }
}
if ((Produtos-Relacionados $UpgradeCode) -ne 0) { Falhar 'já há um Assinador deste escopo instalado; o teste parte de uma máquina limpa' }

# --- Instalar, atualizar, conferir -------------------------------------------------------------------
$fotoAntes = Foto-DoRegistro
$maeDaPastaExistia = Test-Path -LiteralPath (Split-Path -Parent $Pasta)
if ($MsiAnterior) {
  Chamar-Msiexec @('/i', "`"$MsiAnterior`"") 'anterior'
  Conferir-Instalado $VersaoAnterior
}
Chamar-Msiexec @('/i', "`"$Msi`"") 'instalar'
Conferir-Instalado $Versao
$instalados = Produtos-Relacionados $UpgradeCode
if ($instalados -ne 1) { Falhar "há $instalados produtos com o UpgradeCode depois da instalação (a atualização deixou a versão anterior?)" }

# --- Desinstalar e conferir que nada ficou -----------------------------------------------------------
Chamar-Msiexec @('/x', "`"$Msi`"") 'desinstalar'
if ((Produtos-Relacionados $UpgradeCode) -ne 0) { Falhar 'o produto continua registrado depois de desinstalado' }
if (Test-Path -LiteralPath $Pasta) { Falhar "a pasta $Pasta ficou" }
if ((Test-Path -LiteralPath (Split-Path -Parent $Pasta)) -ne $maeDaPastaExistia) { Falhar "a pasta $(Split-Path -Parent $Pasta) mudou de existência" }
foreach ($folha in $Folhas.Keys) {
  if (Test-Path -LiteralPath "$Colmeia\$folha") { Falhar "a chave $Colmeia\$folha ficou" }
}
$fotoDepois = Foto-DoRegistro
$sumiu = @($fotoAntes | Where-Object { -not $fotoDepois.Contains($_) } | Sort-Object)
$sobrou = @($fotoDepois | Where-Object { -not $fotoAntes.Contains($_) } | Sort-Object)
if ($sumiu.Count -gt 0) { Falhar ("a desinstalação apagou o que existia antes:`n" + ($sumiu -join "`n")) }
if ($sobrou.Count -gt 0) { Falhar ("a desinstalação deixou chaves que não existiam antes:`n" + ($sobrou -join "`n")) }
Write-Output "ok: o MSI $Escopo instala, atualiza, conversa como o navegador e desinstala sem deixar nada"
