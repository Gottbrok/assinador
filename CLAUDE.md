# CLAUDE.md · Assinador

Guia para quem trabalha neste repositório, gente ou agente. Ele é lido INTEIRO em toda sessão:
cada regra aqui muda o que se faz. Registro do que já foi feito vai para o `CHANGELOG.md`.

⚠️ **Este repositório é PÚBLICO.** Nada de segredo, chave privada, token, endereço de infraestrutura
interna, nome de cliente ou dado pessoal em arquivo, commit, issue ou log de CI. Na dúvida, não entra.

## O que é

O Assinador deixa uma página web assinar com o certificado digital A3 da pessoa (token ou cartão
ICP-Brasil) sem que a chave saia do dispositivo. Três peças:

| Peça | Pasta | O que é |
|---|---|---|
| Programa nativo | `nativo/` | Binário em Go, um por sistema. Lê certificados e assina um RESUMO com a chave do cartão ou token. Fala só com a extensão, por native messaging (entrada e saída padrão), sem porta de rede |
| Extensão | `extensao/` | Manifest V3 para Chrome, Edge e Firefox. Liga a página ao programa e mostra a janela de confirmação |
| Instaladores | `instaladores/` | MSI (Windows), `.deb` e `.rpm` (Linux), `.pkg` (macOS, depois) |
| Protocolo e chaves públicas | `protocolo/` | `chaves-publicas.json` (chaves PÚBLICAS dos emissores de bilhete) e as fixtures copiadas da biblioteca |
| Ferramentas | `ferramentas/` | Código de prova e de medição. Nunca vai para release |

Quem usa: o Confidata (`*.confidata.app`) e o ushield (`ushield.app`). O formato do bilhete, o
adapter do navegador e os códigos de erro são definidos UMA vez na biblioteca `@confidata/icp-brasil`
e espelhados aqui, com teste que compara as duas listas pelas fixtures.

**Nomes internos fixos** (trocá-los depois quebra instalações): host de native messaging
`br.com.confidata.assinador`; ID da extensão no Firefox `assinador@confidata.com.br`; módulo Go
`github.com/Gottbrok/assinador/nativo`; os `UpgradeCode` do MSI, um por escopo (por usuário e por
máquina), os mesmos nas duas arquiteturas e no MSI de produção, e os GUIDs dos componentes, todos em
`instaladores/windows/Assinador.wxs`.

## Regras que não se negociam

1. **O programa só assina com bilhete conferido.** O bilhete é um JWS ES256 emitido pelo servidor que
   preparou o resumo. O programa confere, nesta ordem, e recusa na primeira falha: `alg` e `typ`;
   `kid` entre as chaves pinadas; a assinatura; `v`; `iss` igual ao emissor da chave; `aud` igual à
   origem que a extensão informou e dentro dos padrões daquele emissor; `dig` igual ao resumo pedido;
   `cer` igual ao SHA-256 do certificado escolhido; tempo entre `iat - 900 s` e `exp + 900 s`. Quem
   confere é o PROGRAMA, não a extensão, e ele confere de novo em `assinar` (não guarda estado).
2. **PIN nunca em log, disco ou resposta.** PIN em `[]byte`, zerado logo após o login. Nunca repita
   login com PIN errado sozinho: um erro de PIN encerra a operação e devolve `pin-incorreto` com o aviso
   de tentativas. No Windows o PIN é do diálogo do PROVEDOR (o programa nunca o vê), e há provedor
   que o pede de novo depois de um erro sem devolver a recusa: ali o programa encerra quando o
   provedor devolve `SCARD_W_WRONG_CHV`, e quem conta as tentativas é o diálogo (medição da F6a).
3. **Nenhum acesso de rede**, no programa nem na extensão, em nenhum modo. Nenhuma localização: o
   programa não pede permissão de localização e a extensão não declara geolocalização.
4. **Catálogo de módulos e de ATRs só com entrada MEDIDA**, cada uma com a data e quem mediu. Módulo
   novo entra quando alguém instalar o middleware e medir, nunca por suposição.
5. **Build de release não leva chave `dev` nem `teste`.** `nativo/internal/bilhete/chaves.go` é GERADO
   de `protocolo/chaves-publicas.json`, e há catraca em teste. Chave privada de desenvolvimento nunca
   entra no repositório: o build com a tag `dev` lê a chave pública de
   `~/.config/confidata-assinador/chaves-dev.json`.
6. **O protocolo muda só junto com a biblioteca.** Operação, campo ou código de erro novo nasce na
   `@confidata/icp-brasil`, com fixture, e só então entra aqui. JSON estrito dos dois lados: campo
   desconhecido é `protocolo`.
7. **Só RSA na v1.** Lista certificado com chave privada e `keyUsage` com `digitalSignature` ou
   `nonRepudiation`; certificado vencido é LISTADO e nunca assinado. O programa não interpreta campo
   ICP-Brasil: quem lê nome, CPF e empresa do certificado é a biblioteca, no navegador.
8. **Cada módulo PKCS#11 roda num processo filho.** Biblioteca de fabricante que derruba o processo
   derruba só o filho. No Windows não há PKCS#11: o CSP e o KSP do fabricante rodam DENTRO do
   programa, pela API do sistema (`nativo/internal/windows`, §3.5 do plano), e por isso lá o canal
   precisa da regra 12 inteira.
9. **Diagnóstico sem CPF.** O CN ICP-Brasil é `NOME:CPF`; em relatório, log e saída de ferramenta os
   dígitos saem mascarados.
10. **O PIN nunca vira `string`.** O pedido da extensão é lido por `nativo/internal/mensagens`
    (leitor JSON estrito próprio), que entrega o PIN num `[]byte` de capacidade fixa. 🚫 Decodificar
    pedido da extensão com `encoding/json` (v1 ou v2): os dois passam o texto por buffers e `string`
    que ninguém zera. O PIN vai ao filho do módulo num quadro próprio, em bytes, nunca em JSON.
11. **`C_Login` só por `entrarNoToken`** (`nativo/internal/pkcs11/login.go`, que copia o PIN para
    memória do C e a zera antes do `free`). 🚫 O `Login` do `miekg/pkcs11` no programa: ele usa
    `C.CString` e não zera (achado da F0). Só o apoio de teste do SoftHSM o usa, para montar o token.
12. **A saída padrão é do canal.** No modo host, o `main` aponta o DESCRITOR 1 para `/dev/null`
    (`separarCanal`) e só o host escreve na cópia do descritor verdadeiro: o que o C escreve (o
    pcsc-lite roda no processo do host) cai no vazio. O filho de módulo fala pelos descritores 3 e 4,
    com a saída padrão em `/dev/null`. No Windows (`canal_windows.go`), a saída padrão do PROCESSO vai
    para `NUL` (o C runtime de toda DLL carregada depois a lê ao iniciar) e o descritor 1 dos C
    runtimes que já estavam carregados (`msvcrt`, `ucrtbase`) também, por `_dup2`: trocar só o
    `os.Stdout` do Go não protege do `printf` de um CSP. 🚫 Carregar biblioteca PKCS#11 no processo
    do host; 🚫 mandar comando ao cartão pelo PC/SC (ele serve só para ler o estado das leitoras e o
    ATR).
13. **Chaves, IDs e fixtures têm um só escritor.** `protocolo/chaves-publicas.json` lista só chave
    de produção, e `nativo/internal/bilhete/chaves.go` sai dele por `go generate ./internal/bilhete`
    (🚫 à mão). As fixtures do bilhete vêm da biblioteca por `git archive` da tag, com as somas em
    `protocolo/fixtures/ORIGEM.md` (🚫 editar fixture aqui). Os IDs de extensão que o programa aceita
    (`origem.ExtensoesChrome`) são os que vão aos manifestos (`cmd/manifestos`): 🚫 escrever
    manifesto à mão. O ID de desenvolvimento só existe no build `dev`.
14. **Pacote de desenvolvimento não é release.** Os `.deb` e `.rpm` de `instaladores/linux` e os MSI
    de `instaladores/windows` levam o build `dev`; o de produção (sem a tag, com os IDs das lojas,
    assinado) é da F7a. Mudou o pacote, rode `instaladores/linux/testar-pacotes.sh`; mudou o MSI, o
    `instaladores/windows/testar-instalador.ps1` roda no CI (os dois escopos, as duas arquiteturas,
    a atualização por cima da versão anterior): a remoção não pode deixar arquivo, pasta nem chave de
    registro. No MSI, os manifestos apontam o programa pelo NOME (`cmd/manifestos -relativo`, na
    mesma pasta), porque a pasta por usuário só existe na hora da instalação e o MSI não reescreve
    JSON. O MSI tem DUAS versões, como o Linux: a do pacote (`-VersaoDoMsi`, abaixo da primeira
    publicação, para o de produção atualizar o de desenvolvimento) e a que o programa informa
    (`-VersaoDoPrograma`, a da PRÓXIMA publicação, que a biblioteca compara com a versão mínima):
    🚫 nunca a do pacote no programa, senão a página diz à pessoa que o Assinador está desatualizado.
    O `empacotar.ps1` roda os ICE (`wix msi validate`): no WiX 5 o `wix build` não valida.
15. **A extensão só LIGA; quem decide é o programa e a pessoa.** Ela nunca confere o bilhete (só a
    forma) e as janelas mostram só o que veio do PROGRAMA (o bilhete conferido, o certificado) e do
    NAVEGADOR (a origem de quem pediu), nunca o que a página declara. A permissão é por ORIGEM, em
    `storage.local`, uma chave por origem (🚫 `storage.sync`; 🚫 mapa numa chave só, que o fundo e as
    opções reescreviam um por cima do outro). Cada operação tem um orçamento abaixo do prazo da
    página, e passo novo num fluxo usa o que RESTA dele (`prazoDoPasso`), nunca um teto próprio
    somado. Cada pedido da página tem a PORTA dele, e quem espera a pessoa corre contra a saída da
    página e a queda do programa. Pedido que toca o cartão passa pela fila, e janela nova passa pelo
    embargo. Botão de janela de decisão só pela trava de `janela.ts`, e o foco nunca nasce em
    "Assinar" nem em "Permitir". 🚫 Permissão nova no manifesto, `host_permissions` ou qualquer
    `fetch` (o build reprova rede no bundle). O que a extensão entrega à PÁGINA está declarado no
    `data_collection_permissions` do Firefox: dado novo entregue à página muda a declaração no mesmo
    commit. Texto visível só em `_locales/`, nos dois idiomas (o teste de paridade reprova chave
    faltando ou sobrando); a exceção é o texto do diagnóstico, artefato do suporte que sai em
    português como o do programa.

## Como se trabalha aqui

- **Commit pequeno e com pathspec explícito**: `git commit -m "msg" -- <caminhos>`. Nunca `git add .`
  nem `git add -A`. Mensagem em português, sem travessão (nem `—` nem `–`).
- **Push, release e publicação nas lojas são do Cairo**, sempre com pedido explícito.
- **Nunca `sed`, `awk` ou script de substituição em código.** Edição arquivo por arquivo.
- **Agentes em paralelo não editam** este repositório ao mesmo tempo.
- **Nunca** `git reset`, `git push --force`, `git rebase` nem `--amend` sobre commit que não é seu.
- Texto visível à pessoa em português do Brasil (na extensão, também em espanhol, pelo
  `_locales/es`). Nomes em código também em português, como na biblioteca (`conferirBilhete`,
  `resumoParaExibicao`).
- Go na versão estável corrente, fixada em cada `go.mod`. Rode `go vet` e `go test ./...` no módulo
  tocado antes de commitar.
- **No `nativo/`, os dois builds**: `go vet ./... && go vet -tags dev ./...`, `go test ./...` e
  `go test -tags dev ./...`, e o `staticcheck` e o `govulncheck` nas versões do
  `.github/workflows/nativo.yml`. Os testes com SoftHSM2 pulam sem ele: aponte o `.so` em
  `ASSINADOR_SOFTHSM` (sem root: `apt download softhsm2 libsofthsm2 softhsm2-common` e `dpkg -x`).
  `ASSINADOR_EXIGE_SOFTHSM=1` (o CI liga) faz a falta reprovar.
- **O Windows se testa no CI** (job `windows`, x64 e arm64): os testes do provedor criam
  certificados com chave de software pelo `New-SelfSignedCertificate` e os apagam no fim
  (`internal/windowsteste`); `ASSINADOR_EXIGE_WINDOWS=1` faz a falta reprovar. Fora do Windows,
  rode `GOOS=windows go vet ./...` (x64 e arm64, os dois builds) e o `staticcheck` com `GOOS=windows`
  a partir do binário da máquina (`go install`, e não `go run`, que o compila para o Windows). Os
  testes de cabeçalho do Windows (`pcsc_cabecalho`, `windows_cabecalho`) compilam aqui com o MinGW
  extraído sem root (`gcc-mingw-w64-x86-64-win32`, `mingw-w64-x86-64-dev` e dependências) e o
  `CC` apontando para ele; rodar, só no Windows. Teste que depende do sistema (a frase do diagnóstico,
  um caminho absoluto) fixa o sistema ou usa a forma dele: a suíte diz o mesmo no Linux e no Windows.
  O MSI se monta e se prova só no Windows: fora dele, o `wix build` para nos caminhos (`WIX0389`,
  `WIX0027`) e, sem eles, no `msi.dll`. As TABELAS se conferem aqui com uma variante do `.wxs` sem as
  pastas e os arquivos, compilada com `-outputtype intermediatepostlink` (pré-processador, linker,
  atualização, resumo do pacote). Os scripts `.ps1` se conferem com o `pwsh` (análise da sintaxe, e as
  funções carregadas pela árvore de sintaxe, com o COM trocado por um falso). ⚠️ No PowerShell, objeto
  COM enumerável que sai de função é DESENROLADO (o StringList do `RelatedProducts` vira `$null` ou
  texto): resultado de `InvokeMember` sai com a vírgula (`, $x`), e lista devolvida assim se lê para
  uma variável, nunca por `@(...)`, que a embrulharia de novo.
- **Na `extensao/`**: `npm run type-check`, `npm test`, `npm run reproduzivel` e `npm run
  lint:firefox`; o `npm run ponta-a-ponta` (Chromium do Playwright, programa dev e SoftHSM2) abre
  navegador, então pergunte ao Cairo antes de rodá-lo fora do CI.

## Estado

As fases e o que cada uma entrega estão no plano de referência do produto (mantido fora deste
repositório). O que já foi medido com cartão real fica em `docs/medicoes/`, com data e equipamento.
