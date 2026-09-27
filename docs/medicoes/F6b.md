# Medições da F6b

Cada medição tem data, equipamento e quem mediu. O que não foi medido fica marcado como pendente,
nunca presumido.

## Medido

**2026-09-26 · Claude, sem Windows**

- **Os scripts** (`empacotar.ps1` e `testar-instalador.ps1`) passam na análise de sintaxe do
  PowerShell 7.6.6 para Linux. As funções que não tocam no Windows Installer rodaram aqui: as
  chaves-mãe de cada chave de navegador saem de `Software\<fabricante>` até `NativeMessagingHosts`;
  a foto do registro vazia (a de um Windows sem navegador) é um conjunto, e não falha; e a conversa
  com o programa pelo quadro de native messaging, contra o programa `dev` do Linux, devolveu o olá
  com o `id` pedido e a versão do build.
- **O gerador de manifestos com `-relativo`** escreve `"path": "assinador.exe"` nos dois manifestos
  e recusa pasta, `..`, os dois separadores e letra de unidade (`C:assinador.exe` é relativo à pasta
  corrente da unidade, e não à do manifesto). Compila para `windows/amd64` e `windows/arm64`.
- **O WiX 5.0.2** roda no .NET 8.0.31 (`rollForward: Major` no `wix.runtimeconfig.json`, sobre o
  `net6.0`). Fora do Windows ele não monta MSI: o `wix build` passa o pré-processador (as duas
  ramificações de escopo, e o escopo inválido recusado com a mensagem do fonte) e para nos caminhos
  (`WIX0389` no nome das pastas, `WIX0027` na origem dos arquivos), que ele só aceita na forma do
  Windows. Montar, validar (os ICE) e instalar é do CI.

## Pendente

- [ ] **O primeiro CI da F6b** (job `windows`, x64 e arm64): os dois MSI montados com os ICE limpos
      (o ICE38 e o ICE64 do MSI por usuário: chave `HKCU` como caminho-chave e a remoção das pastas
      do perfil), e o `testar-instalador.ps1` verde nos dois escopos, com a atualização por cima da
      0.0.1. Se o Windows Installer deixar vazia uma chave-mãe que ele criou (por exemplo
      `HKCU\Software\Chromium`), a prova reprova, e a correção é decidida a partir do que ficou.
- [ ] **Windows 10 e 11, com o Cairo**, junto do gate da F6a:
  - o MSI por usuário, baixado do artefato, instala numa conta SEM administrador, sem pedido de
    elevação; anotar o que o SmartScreen mostra no MSI sem assinatura (para comparar com o assinado
    da F6b-ii);
  - com a extensão de desenvolvimento, a página de teste encontra o programa no Chrome, no Edge e no
    Firefox (o olá e a lista de certificados);
  - a versão nova por cima da anterior mantém tudo funcionando, e a desinstalação por
    "Configurações, Aplicativos" não deixa a pasta nem as chaves;
  - o MSI por máquina por `msiexec /i ... /qn` num prompt de administrador, e os navegadores de
    outra conta do mesmo computador encontram o programa.
- [ ] **F6b-ii, com o certificado de assinatura de código:** Authenticode com carimbo do tempo no
      `.exe` e no `.msi`, no CI; `signtool verify /pa` aceita os dois; o SmartScreen no primeiro
      download do MSI assinado, com captura.
- [ ] **Na publicação:** as chaves de extensão externa (`Software\Google\Chrome\Extensions\<id>` com
      o `update_url` da loja) fazem o Chrome e o Edge oferecerem a extensão depois do MSI? Precisa do
      ID da loja.
