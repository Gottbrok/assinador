<#
Prova um MSI de DESENVOLVIMENTO do Assinador no Windows (F6b), sem cartão: o par do
`instaladores/linux/testar-pacotes.sh`. Confere a estrutura do pacote; fotografa as chaves de native
messaging dos navegadores e as pastas; (com -MsiAnterior) instala a versão anterior e prova a
ATUALIZAÇÃO por cima dela; confere o produto instalado, o programa, os manifestos e as chaves; resolve o
programa como o navegador resolve (a chave aponta o manifesto, o manifesto aponta o programa pelo nome,
na pasta dele) e conversa com ele pelo protocolo do native messaging, com os argumentos que o Chrome
passa no Windows; desinstala; e reprova se o que ficou não for o que havia antes. Abrir pelo navegador
de verdade fica para o teste manual.

O MSI por usuário prova, pela ESTRUTURA, que não pede administrador: o bit de "privilégio elevado não
exigido" do resumo do pacote, o ALLUSERS vazio, todo valor de registro em HKCU e nenhuma pasta de
máquina. Instalar numa conta sem administrador é do teste manual: o executor do CI é administrador. O
por máquina precisa de administrador (grava em Program Files e HKLM).

  .\instaladores\windows\testar-instalador.ps1 -Msi dist\assinador-dev-windows-amd64-usuario.msi -Escopo usuario -VersaoDoPrograma 1.0.0 -MsiAnterior anterior\assinador-dev-windows-amd64-usuario.msi -VersaoAnteriorDoPrograma 0.0.1

`-VersaoDoPrograma` é a que o programa instalado informa (a `-VersaoDoPrograma` do `empacotar.ps1`); a
versão do PRODUTO instalado é conferida contra a do próprio MSI. Os logs do msiexec vão para
`-PastaDeLogs` (sem ela, para a pasta temporária).

No PowerShell 7 (`pwsh`): o 5.1 não tem o `ProcessStartInfo.ArgumentList` e lê o script sem BOM como
ANSI.
#>
#Requires -Version 7.2
param(
  [Parameter(Mandatory)][string]$Msi,
  [Parameter(Mandatory)][ValidateSet('usuario', 'maquina')][string]$Escopo,
  [Parameter(Mandatory)][string]$VersaoDoPrograma,
  [string]$MsiAnterior,
  [string]$VersaoAnteriorDoPrograma,
  [string]$PastaDeLogs
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ([bool]$MsiAnterior -ne [bool]$VersaoAnteriorDoPrograma) { throw 'Informe -MsiAnterior e -VersaoAnteriorDoPrograma juntos.' }
$Raiz = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$Msi = (Resolve-Path -LiteralPath $Msi).Path
if ($MsiAnterior) { $MsiAnterior = (Resolve-Path -LiteralPath $MsiAnterior).Path }
if (-not $PastaDeLogs) { $PastaDeLogs = Join-Path ([System.IO.Path]::GetTempPath()) 'assinador-msiexec' }
New-Item -ItemType Directory -Force -Path $PastaDeLogs | Out-Null
$PastaDeLogs = (Resolve-Path -LiteralPath $PastaDeLogs).Path
$Nome ='br.com.confidata.assinador'
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
$Maes = @($Folhas.Keys | ForEach-Object { Maes-DaFolha $_ } | Sort-Object -Unique)

# A foto do que o navegador lê: a existência de cada chave-mãe e tudo o que há sob cada
# `NativeMessagingHosts` (chaves e valores, de qualquer programa). Fotografar a árvore inteira do
# fabricante pegaria o que o próprio navegador ou o atualizador dele escrevem durante o teste.
function Foto-DoRegistro {
  $linhas = [System.Collections.Generic.List[string]]::new()
  foreach ($mae in $Maes) {
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

# A chave não guarda valor nenhum em toda a subárvore (só chaves vazias). O Windows Installer apaga a
# chave que fica vazia depois de ele remover o último valor ou a última subchave dela, sem olhar quem a
# criou: uma chave-mãe assim, que já existia, some na desinstalação, e isso não é sobra nem estrago.
# Para no primeiro valor achado (a `Software\Microsoft` de HKLM é enorme), e chave que não abre conta
# como tendo valor.
function Subarvore-SemValores([Microsoft.Win32.RegistryKey]$chave) {
  if ($chave.ValueCount -gt 0) { return $false }
  foreach ($nome in $chave.GetSubKeyNames()) {
    try { $filha = $chave.OpenSubKey($nome) } catch { return $false }
    if ($null -eq $filha) { return $false }
    try { if (-not (Subarvore-SemValores $filha)) { return $false } } finally { $filha.Dispose() }
  }
  $true
}

# A pasta-mãe da pasta do programa: ausente, vazia ou com conteúdo. A desinstalação remove a pasta que
# ficar vazia (o `RemoveFolder` do MSI por usuário), então a vazia pode sumir.
function Estado-DaPasta([string]$caminho) {
  if (-not (Test-Path -LiteralPath $caminho)) { return 'ausente' }
  if (Get-ChildItem -LiteralPath $caminho -Force | Select-Object -First 1) { return 'com-conteudo' }
  'vazia'
}

function Chamar-Msiexec([string[]]$argumentos, [string]$rotulo) {
  $log = Join-Path $PastaDeLogs "msiexec-$Escopo-$rotulo.log"
  # 1618: outra instalação em curso (no executor, às vezes um serviço do sistema). Tenta de novo.
  for ($tentativa = 1; ; $tentativa++) {
    $p = Start-Process -FilePath 'msiexec.exe' -ArgumentList ($argumentos + @('/qn', '/norestart', '/l*v', "`"$log`"")) -Wait -PassThru
    if ($p.ExitCode -ne 1618 -or $tentativa -ge 6) { break }
    Write-Output "msiexec ($rotulo): outra instalação em curso, nova tentativa em 20 s"
    Start-Sleep -Seconds 20
  }
  if ($p.ExitCode -notin 0, 3010) {
    # O motivo fica perto do primeiro "Return value 3" (a ação que falhou), e não no fim do log.
    $linhas = @(Get-Content -LiteralPath $log -ErrorAction SilentlyContinue)
    $falha = $linhas | Select-String -SimpleMatch 'Return value 3' | Select-Object -First 1
    if ($falha) { $linhas[[Math]::Max(0, $falha.LineNumber - 40)..[Math]::Min($linhas.Count - 1, $falha.LineNumber + 5)] | Write-Output }
    else { $linhas | Select-Object -Last 80 | Write-Output }
    Falhar "o msiexec ($rotulo) saiu com $($p.ExitCode); log em $log"
  }
}

# O WindowsInstaller.Installer é COM sem biblioteca de tipos para o PowerShell: tudo por InvokeMember.
$Instalador = New-Object -ComObject WindowsInstaller.Installer
function Chamar([object]$objeto, [string]$membro, [System.Reflection.BindingFlags]$tipo, [object[]]$argumentos) {
  # A vírgula entrega o resultado INTEIRO: o PowerShell desenrola objeto COM enumerável que sai de
  # função (o StringList do RelatedProducts), e zero produtos viraria $null e um viraria o texto do
  # ProductCode.
  , $objeto.GetType().InvokeMember($membro, $tipo, $null, $objeto, $argumentos)
}
# A primeira coluna de cada linha de uma consulta ao banco do MSI, como texto.
function Valores-DoMsi([string]$caminho, [string]$consulta) {
  $banco = Chamar $Instalador 'OpenDatabase' InvokeMethod @($caminho, 0)
  $visao = Chamar $banco 'OpenView' InvokeMethod @($consulta)
  Chamar $visao 'Execute' InvokeMethod $null | Out-Null
  $valores = [System.Collections.Generic.List[string]]::new()
  while ($true) {
    $registro = Chamar $visao 'Fetch' InvokeMethod $null
    if ($null -eq $registro) { break }
    $valores.Add((Chamar $registro 'StringData' GetProperty @(1)))
    [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($registro)
  }
  Chamar $visao 'Close' InvokeMethod $null | Out-Null
  # Solta o arquivo: o msiexec o abre depois.
  foreach ($o in $visao, $banco) { [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($o) }
  , $valores.ToArray()
}
# As funções que devolvem lista a entregam inteira (a vírgula): leia-as para uma variável, nunca por
# `@(...)`, que embrulharia a lista num array de um elemento só.
function Propriedade-DoMsi([string]$caminho, [string]$propriedade) {
  $valores = Valores-DoMsi $caminho "SELECT ``Value`` FROM ``Property`` WHERE ``Property`` = '$propriedade'"
  if ($valores.Count -gt 0) { $valores[0] } else { $null }
}
# Os ProductCode instalados (por este usuário ou na máquina) com o UpgradeCode.
function Produtos-Relacionados([string]$upgradeCode) {
  $lista = Chamar $Instalador 'RelatedProducts' GetProperty @($upgradeCode)
  $codigos = [System.Collections.Generic.List[string]]::new()
  $total = Chamar $lista 'Count' GetProperty $null
  for ($i = 0; $i -lt $total; $i++) { $codigos.Add((Chamar $lista 'Item' GetProperty @($i))) }
  [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($lista)
  , $codigos.ToArray()
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

# Conversa com o programa como o Chrome conversa no Windows: lançado com a origem da extensão e o
# `--parent-window` (zero, o do contexto de fundo), um quadro de 4 bytes (o tamanho, na ordem da
# máquina) mais o JSON na entrada, e o mesmo na saída. O `ola` responde a qualquer origem.
function Conversar-Ola([string]$programa) {
  $inicio = [System.Diagnostics.ProcessStartInfo]::new($programa)
  $inicio.ArgumentList.Add("chrome-extension://$IdDev/")
  $inicio.ArgumentList.Add('--parent-window=0')
  $inicio.UseShellExecute = $false
  $inicio.RedirectStandardInput = $true
  $inicio.RedirectStandardOutput = $true
  $inicio.RedirectStandardError = $true
  $p = [System.Diagnostics.Process]::Start($inicio)
  # O erro padrão lido em paralelo: um aviso que encha o buffer dele não trava o programa.
  $erro = $p.StandardError.ReadToEndAsync()
  try {
    $corpo = [System.Text.Encoding]::UTF8.GetBytes('{"v":1,"id":"msi-ola","op":"ola","origem":"http://localhost"}')
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
    if (-not $p.WaitForExit(15000)) { Falhar 'o programa não saiu depois de a entrada fechar' }
    $resposta
  } catch {
    if (-not $p.HasExited) { $p.Kill() }
    $textoDoErro = if ($erro.Wait(5000)) { $erro.Result.Trim() } else { '(sem o erro padrão)' }
    Falhar "a conversa com $programa falhou: $($_.Exception.Message)`nerro padrão do programa: $textoDoErro"
  } finally {
    if (-not $p.HasExited) { $p.Kill() }
    [void]$erro.Wait(5000)
  }
}

function Conferir-Instalado([string]$msiInstalado, [string]$versaoDoPrograma) {
  # O produto: um só com o UpgradeCode, na versão do MSI que acabou de instalar (a atualização não deixou
  # a anterior ao lado).
  $codigos = Produtos-Relacionados $UpgradeCode
  if ($codigos.Count -ne 1) { Falhar "há $($codigos.Count) produtos com o UpgradeCode (a atualização deixou a versão anterior?)" }
  $versaoDoProduto = Chamar $Instalador 'ProductInfo' GetProperty @($codigos[0], 'VersionString')
  $versaoDoMsi = Propriedade-DoMsi $msiInstalado 'ProductVersion'
  if ($versaoDoProduto -ne $versaoDoMsi) { Falhar "o produto instalado está na versão $versaoDoProduto, e não na $versaoDoMsi do MSI" }

  if (-not (Test-Path -LiteralPath (Join-Path $Pasta 'assinador.exe'))) { Falhar "o programa não está em $Pasta" }
  $manifestoDoChrome = $null
  foreach ($folha in $Folhas.Keys) {
    $chave = "$Colmeia\$folha"
    if (-not (Test-Path -LiteralPath $chave)) { Falhar "a chave $chave não foi criada" }
    $apontado = (Get-Item -LiteralPath $chave).GetValue('')
    $esperado = Join-Path $Pasta $Folhas[$folha]
    if ($apontado -ne $esperado) { Falhar "a chave $chave aponta para '$apontado', e não para '$esperado'" }
    if ($folha -like 'Software\Google\Chrome\*') { $manifestoDoChrome = $apontado }
    $manifesto = Get-Content -LiteralPath $apontado -Raw | ConvertFrom-Json
    if ($manifesto.name -ne $Nome -or $manifesto.type -ne 'stdio') { Falhar "manifesto $apontado fora da forma" }
    if ($manifesto.path -ne 'assinador.exe') { Falhar "o manifesto $apontado aponta '$($manifesto.path)', e não o programa pelo nome" }
    if ($Folhas[$folha] -eq 'firefox.json') {
      if (@($manifesto.allowed_extensions) -notcontains 'assinador@confidata.com.br') { Falhar 'o manifesto do Firefox não traz a extensão' }
    } elseif (@($manifesto.allowed_origins) -notcontains "chrome-extension://$IdDev/") {
      Falhar "o manifesto $apontado não traz a extensão de desenvolvimento"
    }
  }
  # O programa como o Chrome o acha: o `path` do manifesto, na pasta do manifesto.
  $programa = Join-Path (Split-Path -Parent $manifestoDoChrome) (Get-Content -LiteralPath $manifestoDoChrome -Raw | ConvertFrom-Json).path
  $informada = (& $programa versao | Out-String).Trim()
  if ($LASTEXITCODE -ne 0 -or $informada -ne $versaoDoPrograma) { Falhar "o programa instalado informa '$informada', e não $versaoDoPrograma" }
  $ola = Conversar-Ola $programa
  if (-not $ola.ok -or $ola.id -ne 'msi-ola' -or $ola.dados.versao -ne $versaoDoPrograma) { Falhar "o olá do programa instalado veio fora do esperado: $($ola | ConvertTo-Json -Compress -Depth 5)" }
  Write-Output "instalado: produto $versaoDoProduto; $programa informa $informada e responde ao olá (plataforma $($ola.dados.plataforma))"
}

# --- A estrutura do pacote ---------------------------------------------------------------------------
$UpgradeCode = Propriedade-DoMsi $Msi 'UpgradeCode'
if (-not $UpgradeCode) { Falhar 'o MSI não tem UpgradeCode' }
if ($MsiAnterior -and (Propriedade-DoMsi $MsiAnterior 'UpgradeCode') -ne $UpgradeCode) { Falhar 'o MSI anterior tem outro UpgradeCode' }
$raizes = Valores-DoMsi $Msi 'SELECT `Root` FROM `Registry`'
$pastas = Valores-DoMsi $Msi 'SELECT `Directory` FROM `Directory`'
if ($raizes.Count -eq 0 -or $pastas.Count -eq 0) { Falhar 'o MSI não tem valores de registro ou pastas: a consulta ao banco dele não leu nada' }
if ($Escopo -eq 'usuario') {
  $resumo = Chamar $Instalador 'SummaryInformation' GetProperty @($Msi, 0)
  $contagem = [int](Chamar $resumo 'Property' GetProperty @(15))
  [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($resumo)
  # PID_WORDCOUNT, bit 8: o pacote não exige privilégio elevado (o Windows não pede UAC).
  if (($contagem -band 8) -eq 0) { Falhar "o MSI por usuário exige privilégio elevado (Word Count $contagem)" }
  if ((Propriedade-DoMsi $Msi 'ALLUSERS') -in '1', '2') { Falhar 'o MSI por usuário declara ALLUSERS' }
  # Uma linha em HKLM ou uma pasta de máquina passariam aqui (o executor é administrador) e dariam erro
  # 1925 numa conta comum: a estrutura as recusa.
  if (@($raizes | Where-Object { $_ -ne '1' }).Count -gt 0) { Falhar "o MSI por usuário grava fora de HKCU (Root: $($raizes -join ', '))" }
  $deMaquina = 'ProgramFilesFolder', 'ProgramFiles64Folder', 'ProgramFiles6432Folder', 'CommonFilesFolder', 'CommonFiles64Folder', 'CommonFiles6432Folder', 'CommonAppDataFolder', 'SystemFolder', 'System64Folder', 'System6432Folder', 'WindowsFolder'
  $invasoras = @($pastas | Where-Object { $_ -in $deMaquina })
  if ($invasoras.Count -gt 0) { Falhar "o MSI por usuário usa pasta de máquina: $($invasoras -join ', ')" }
} else {
  if (@($raizes | Where-Object { $_ -ne '2' }).Count -gt 0) { Falhar "o MSI por máquina grava fora de HKLM (Root: $($raizes -join ', '))" }
  $doPerfil = 'LocalAppDataFolder', 'AppDataFolder', 'PersonalFolder'
  $invasoras = @($pastas | Where-Object { $_ -in $doPerfil })
  if ($invasoras.Count -gt 0) { Falhar "o MSI por máquina usa pasta do perfil: $($invasoras -join ', ')" }
}
if ((Produtos-Relacionados $UpgradeCode).Count -ne 0) { Falhar 'já há um Assinador deste escopo instalado; o teste parte de uma máquina sem ele' }

# --- Instalar, atualizar, conferir -------------------------------------------------------------------
$fotoAntes = Foto-DoRegistro
$maesSemValores = @($Maes | Where-Object {
    $k = Get-Item -LiteralPath "$Colmeia\$_" -ErrorAction SilentlyContinue
    $k -and (Subarvore-SemValores $k)
  })
$PastaMae = Split-Path -Parent $Pasta
$estadoDaPastaMae = Estado-DaPasta $PastaMae
if ($MsiAnterior) {
  Chamar-Msiexec @('/i', "`"$MsiAnterior`"") 'anterior'
  Conferir-Instalado $MsiAnterior $VersaoAnteriorDoPrograma
}
Chamar-Msiexec @('/i', "`"$Msi`"") 'instalar'
Conferir-Instalado $Msi $VersaoDoPrograma

# --- Desinstalar e conferir que nada ficou -----------------------------------------------------------
Chamar-Msiexec @('/x', "`"$Msi`"") 'desinstalar'
if ((Produtos-Relacionados $UpgradeCode).Count -ne 0) { Falhar 'o produto continua registrado depois de desinstalado' }
if (Test-Path -LiteralPath $Pasta) { Falhar "a pasta $Pasta ficou" }
$estadoDepois = Estado-DaPasta $PastaMae
if ($estadoDaPastaMae -ne 'vazia' -and $estadoDepois -ne $estadoDaPastaMae) { Falhar "a pasta $PastaMae estava $estadoDaPastaMae e ficou $estadoDepois" }
foreach ($folha in $Folhas.Keys) {
  if (Test-Path -LiteralPath "$Colmeia\$folha") { Falhar "a chave $Colmeia\$folha ficou" }
}
$fotoDepois = Foto-DoRegistro
$sumiu = @($fotoAntes | Where-Object { -not $fotoDepois.Contains($_) } | Sort-Object)
$sobrou = @($fotoDepois | Where-Object { -not $fotoAntes.Contains($_) } | Sort-Object)
# O que sumiu por ser chave vazia (a regra do Windows Installer, acima) não conta.
$vazias = @($maesSemValores | ForEach-Object { "$NomeDaColmeia\$_" })
$apagadasPorRegra = @($sumiu | Where-Object {
    $linha = $_
    @($vazias | Where-Object { $linha -eq "existe $_" -or $linha -eq "chave $_" -or $linha -like "chave $_\*" }).Count -gt 0
  })
$sumiu = @($sumiu | Where-Object { $_ -notin $apagadasPorRegra })
if ($apagadasPorRegra.Count -gt 0) { Write-Output ("a desinstalação apagou chaves vazias que já existiam (regra do Windows Installer):`n" + ($apagadasPorRegra -join "`n")) }
if ($sumiu.Count -gt 0) { Falhar ("a desinstalação apagou o que existia antes:`n" + ($sumiu -join "`n")) }
if ($sobrou.Count -gt 0) { Falhar ("a desinstalação deixou chaves que não existiam antes:`n" + ($sobrou -join "`n")) }
Write-Output "ok: o MSI $Escopo instala, atualiza, conversa como o navegador conversa e desinstala sem deixar nada"
