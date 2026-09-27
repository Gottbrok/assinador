# Medições da F6b

Cada medição tem data, equipamento e quem mediu. O que não foi medido fica marcado como pendente,
nunca presumido.

## Medido

**2026-09-26 · Claude, sem Windows**

- **Os scripts** (`empacotar.ps1` e `testar-instalador.ps1`) passam na análise de sintaxe do
  PowerShell 7.6.6 para Linux. As funções dele, carregadas pela árvore de sintaxe com o COM do
  Windows Installer trocado por um falso, devolvem as consultas ao MSI com zero, uma e duas linhas e
  os produtos relacionados com zero, um e dois; o msiexec que responde 1618 é repetido, e o que falha
  mostra o trecho do log em volta do "Return value 3"; a pasta-mãe sai como ausente, vazia ou com
  conteúdo (arquivo oculto conta); e a tolerância à chave-mãe vazia apagada pela regra do Windows
  Installer não deixa passar nome parecido nem valor alheio. A conversa com o programa pelo quadro de
  native messaging, com `--parent-window=0`, contra o programa `dev` do Linux, devolveu o olá com o
  `id` pedido e a versão do programa; um programa que sai sem responder reprova com o erro padrão
  dele.
- **A vírgula na saída de função** (achado da auditoria): no PowerShell 7, uma coleção que sai de
  função é desenrolada (a vazia vira `$null`), e `, $x` a entrega inteira, vazia, de um ou de vários
  elementos; `@(função)` a embrulha de novo num array de um elemento só. Medido com coleções .NET,
  que o PowerShell trata como os objetos COM enumeráveis.
- **O gerador de manifestos com `-relativo`** escreve `"path": "assinador.exe"` nos dois manifestos
  e aceita só o nome de um `.exe` em lista branca: recusa pasta, `..`, os dois separadores, letra de
  unidade (`C:assinador.exe` é relativo à pasta corrente da unidade, e não à do manifesto), espaço e
  ponto no fim, nome de dispositivo (`nul.exe`, `COM1.exe`), caractere proibido e de controle.
  Compila para `windows/amd64` e `windows/arm64`.
- **O WiX 5.0.2** roda no .NET 8.0.31 (`rollForward: Major` no `wix.runtimeconfig.json`, sobre o
  `net6.0`). Fora do Windows o `wix build` para nos caminhos (`WIX0389` no nome das pastas, `WIX0027`
  na origem dos arquivos) e, sem eles, no `msi.dll`. Uma variante do `.wxs` sem as pastas e os
  arquivos, compilada até o intermediário pós-link (`-outputtype intermediatepostlink`) nos quatro
  casos (por usuário e por máquina, x64 e arm64), mostra as tabelas:
  - o por usuário sem `ALLUSERS` e com Word Count 10 (o bit de "privilégio elevado não exigido" mais
    o comprimido); o por máquina com `ALLUSERS=1` e Word Count 2;
  - o template `x64;1046` e `Arm64;1046`, e o `InstallerVersion` 500, que basta para o arm64;
  - a `ProgramFiles6432Folder` vira `ProgramFiles64Folder`, e o componente é de 64 bits;
  - sem `<Feature>`, o WiX 5 cria a feature padrão sozinho;
  - a linha de atualização sem idioma (`IgnoreLanguage`) e com a versão máxima inclusiva
    (`AllowSameVersionUpgrades`), e a remoção da versão anterior depois do `InstallValidate`;
  - o resumo do pacote em português, na página de código 1252.
  Validar (os ICE, pelo `wix msi validate` do `empacotar.ps1`) e instalar só o Windows faz: é do CI.

## Pendente

- [ ] **O primeiro CI da F6b** (job `windows`, x64 e arm64): os dois MSI montados e validados, com
      três avisos ICE91 esperados no por usuário (arquivos no perfil, inofensivos em pacote só por
      usuário, pela documentação da Microsoft), um ICE61 nos dois (a mesma versão atualiza, de
      propósito) e nenhum erro; e o `testar-instalador.ps1` verde nos
      dois escopos, com a atualização por cima da 0.0.1. A documentação da tabela Registry diz que o
      Windows Installer apaga a chave depois de remover o último valor ou a última subchave dela, então
      as chaves-mãe que o MSI criou devem sumir na desinstalação; o CI confirma.
- [ ] **Windows 10 e 11, com o Cairo**, junto do gate da F6a:
  - o MSI por usuário, baixado do artefato, instala numa conta SEM administrador, sem pedido de
    elevação, e também numa conta com acento no nome (o caminho do perfil entra no manifesto que o
    navegador lê); anotar o que o SmartScreen mostra no MSI sem assinatura (para comparar com o
    assinado da F6b-ii);
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
      ID da loja. E o MSI de produção por máquina não substitui um por usuário de desenvolvimento que
      tenha ficado instalado (o Chrome e o Edge leem `HKCU` antes de `HKLM`): decidir ali como tratar.
