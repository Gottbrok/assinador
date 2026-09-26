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
`github.com/Gottbrok/assinador/nativo`.

## Regras que não se negociam

1. **O programa só assina com bilhete conferido.** O bilhete é um JWS ES256 emitido pelo servidor que
   preparou o resumo. O programa confere, nesta ordem, e recusa na primeira falha: `alg` e `typ`;
   `kid` entre as chaves pinadas; a assinatura; `v`; `iss` igual ao emissor da chave; `aud` igual à
   origem que a extensão informou e dentro dos padrões daquele emissor; `dig` igual ao resumo pedido;
   `cer` igual ao SHA-256 do certificado escolhido; tempo entre `iat - 900 s` e `exp + 900 s`. Quem
   confere é o PROGRAMA, não a extensão, e ele confere de novo em `assinar` (não guarda estado).
2. **PIN nunca em log, disco ou resposta.** PIN em `[]byte`, zerado logo após o login. Nunca repita
   login com PIN errado sozinho: um erro de PIN encerra a operação e devolve `pin-incorreto` com o aviso
   de tentativas.
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
   derruba só o filho.
9. **Diagnóstico sem CPF.** O CN ICP-Brasil é `NOME:CPF`; em relatório, log e saída de ferramenta os
   dígitos saem mascarados.
10. **O PIN nunca vira `string`.** O pedido da extensão é lido por `nativo/internal/mensagens`
    (leitor JSON estrito próprio), que entrega o PIN num `[]byte` de capacidade fixa. 🚫 Decodificar
    pedido da extensão com `encoding/json` (v1 ou v2): os dois passam o texto por buffers e `string`
    que ninguém zera. O PIN vai ao filho do módulo num quadro próprio, em bytes, nunca em JSON.
11. **`C_Login` só por `entrarNoToken`** (`nativo/internal/pkcs11/login.go`, que copia o PIN para
    memória do C e a zera antes do `free`). 🚫 O `Login` do `miekg/pkcs11` no programa: ele usa
    `C.CString` e não zera (achado da F0). Só o apoio de teste do SoftHSM o usa, para montar o token.
12. **A saída padrão é do canal.** No modo host, só o host escreve no descritor da saída padrão (o
    `main` troca o `os.Stdout` por `/dev/null`); o filho de módulo fala pelos descritores 3 e 4, com
    a saída padrão em `/dev/null`. 🚫 Carregar biblioteca PKCS#11 no processo do host.
13. **Chaves e fixtures têm um só escritor.** `protocolo/chaves-publicas.json` lista só chave de
    produção, e `nativo/internal/bilhete/chaves.go` sai dele por `go generate ./internal/bilhete`
    (🚫 à mão). As fixtures do bilhete vêm da biblioteca por `git archive` da tag, com as somas em
    `protocolo/fixtures/ORIGEM.md` (🚫 editar fixture aqui).

## Como se trabalha aqui

- **Commit pequeno e com pathspec explícito**: `git commit -m "msg" -- <caminhos>`. Nunca `git add .`
  nem `git add -A`. Mensagem em português, sem travessão (nem `—` nem `–`).
- **Push, release e publicação nas lojas são do Cairo**, sempre com pedido explícito.
- **Nunca `sed`, `awk` ou script de substituição em código.** Edição arquivo por arquivo.
- **Agentes em paralelo não editam** este repositório ao mesmo tempo.
- **Nunca** `git reset`, `git push --force`, `git rebase` nem `--amend` sobre commit que não é seu.
- Texto visível à pessoa em português do Brasil. Nomes em código também em português, como na
  biblioteca (`conferirBilhete`, `resumoParaExibicao`).
- Go na versão estável corrente, fixada em cada `go.mod`. Rode `go vet` e `go test ./...` no módulo
  tocado antes de commitar.
- **No `nativo/`, os dois builds**: `go vet ./... && go vet -tags dev ./...`, `go test ./...` e
  `go test -tags dev ./...`, e o `staticcheck` e o `govulncheck` nas versões do
  `.github/workflows/nativo.yml`. Os testes com SoftHSM2 pulam sem ele: aponte o `.so` em
  `ASSINADOR_SOFTHSM` (sem root: `apt download softhsm2 libsofthsm2 softhsm2-common` e `dpkg -x`).
  `ASSINADOR_EXIGE_SOFTHSM=1` (o CI liga) faz a falta reprovar.

## Estado

As fases e o que cada uma entrega estão no plano de referência do produto (mantido fora deste
repositório). O que já foi medido com cartão real fica em `docs/medicoes/`, com data e equipamento.
