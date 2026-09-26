# Extensão do Assinador

Manifest V3 para Chrome, Edge e Firefox. Liga a página ao programa que lê o cartão ou o token e
mostra à pessoa, antes de cada assinatura, o documento verdadeiro e o endereço de quem pede. Sem
framework e sem acesso de rede: é superfície de segurança, e fica pequena para ser lida.

O que ela faz, pedido a pedido, e os prazos estão em [`../protocolo/PROTOCOLO.md`](../protocolo/PROTOCOLO.md)
(seção "Página e extensão"). O que ela acessa e o que não faz, em [`PRIVACIDADE.md`](PRIVACIDADE.md).

## Arquivos

| Arquivo | O que é |
|---|---|
| `manifest.base.ts` | O manifesto dos dois alvos, de uma fonte só (`chrome`: service worker; `firefox`: script de fundo e o ID `assinador@confidata.com.br`) |
| `src/conteudo.ts` | A ponte entre a página e o fundo (mundo isolado, quadro de topo) |
| `src/fundo.ts` | Os portões (remetente, origem, permissão, forma), o fluxo de `assinar` e o orçamento de prazos |
| `src/nativo.ts` | A porta de native messaging com o programa, um pedido por vez |
| `src/janelas.ts` e `src/janela.ts` | As janelas de decisão: o lado do fundo e o lado da página da janela (com a trava de 600 ms dos botões) |
| `src/permissoes.ts` | A lista de endereços permitidos, em `storage.local` |
| `src/confirmar.ts`, `src/permitir.ts`, `src/opcoes.ts` e `paginas/` | As três páginas da extensão |
| `_locales/pt_BR` e `_locales/es` | Todo texto visível (o teste de paridade exige as mesmas chaves e marcadores) |
| `icones/` | Ícones PROVISÓRIOS, de `scripts/gerar-icones-provisorios.mjs`; a marca vem com o nome definitivo |
| `e2e/` | O teste no navegador e a página de teste (JavaScript puro, que fala o protocolo à mão) |

## Compilar e testar

Node 22.18 ou mais novo (o `build.mjs` importa o `manifest.base.ts` pela remoção de tipos do Node).

```sh
npm ci
npm run type-check
npm test                  # unidade (Vitest)
npm run build             # dist/chrome e dist/firefox, como vão para a loja
npm run build:dev         # dist/chrome-dev e dist/firefox-dev: localhost e o ID de desenvolvimento
npm run pacotes           # os zips da loja em pacotes/
npm run reproduzivel      # monta os zips duas vezes e confere o SHA-256
npm run lint:firefox      # web-ext lint do build da loja (depois do build)
```

Os scripts saem num arquivo cada, sem minificar. O pacote do Chrome sai SEM `key` (a Chrome Web
Store recusa o campo; o ID vem da loja). O de desenvolvimento leva a chave PÚBLICA de
`../protocolo/extensao-dev.json`, e é ela que fixa o ID que o programa de desenvolvimento aceita.

O pacote é reproduzível: mesmo SHA-256 em dois builds da mesma árvore, com qualquer fuso e qualquer
`NODE_ENV` (o carimbo do zip é a data do commit, ou `SOURCE_DATE_EPOCH`).

O `web-ext lint` deixa um aviso, de propósito: o `data_collection_permissions` pede Firefox 142 no
Android, e o mínimo é o 140 (o ESR que empresa e órgão público usam). No Android não há native
messaging; a extensão é só para computador.

## Ponta a ponta

```sh
npm run ponta-a-ponta
```

Monta a extensão de desenvolvimento e roda `nativo/cmd/assinador/extensao_test.go` (tag
`extensao`), que prepara o token SoftHSM2, o programa de desenvolvimento, a chave dev do bilhete, o
manifesto do host no diretório de dados do Chromium do Playwright e a página de teste
(`host-teste servir`), e então roda `e2e/extensao.spec.ts`: permissão negada e dada, a lista, a
janela de confirmação (com o titular sem CPF), o cancelamento, a assinatura conferida contra o
certificado, o bilhete de outro resumo recusado pelo programa, e as opções. Precisa do Go, do gcc,
do SoftHSM2 (`ASSINADOR_SOFTHSM` aponta o `.so` se não estiver num caminho comum) e do Chromium do
Playwright (`npx playwright install chromium`). `E2E_HEADED=1` mostra o navegador.

## Testar com o cartão, no navegador

1. Programa de desenvolvimento e manifesto do host: o pacote `.deb` de desenvolvimento
   (`../instaladores/linux/empacotar.sh`) instala os dois; ou compile e gere o manifesto com
   `go run -tags dev ./cmd/manifestos` (em `nativo/`) e ponha o `chromium.json` como
   `~/.config/google-chrome/NativeMessagingHosts/br.com.confidata.assinador.json`.
2. `npm run build:dev`, e em `chrome://extensions` (modo de desenvolvedor) "Carregar sem compactação"
   de `dist/chrome-dev`. O ID precisa ser `jmogljhnfdnhhoapijclifkpjfbhhppf`. No Firefox,
   `about:debugging`, "Carregar extensão temporária", o `manifest.json` de `dist/firefox-dev`.
3. A chave dev do bilhete e a página: da raiz do repositório, `bin/host-teste gerar-chave` (uma vez)
   e `bin/host-teste servir`, e abra `http://localhost:8787`.
4. "Procurar certificados" (a janela de permissão aparece na primeira vez), escolha o certificado e
   "Assinar": a janela mostra o documento, o endereço e o certificado, e a resposta diz se o
   servidor conferiu a assinatura.

O Chrome de marca não carrega extensão por linha de comando desde a versão 137; carregar pela tela de
extensões funciona. O Firefox e o Chromium em Snap ainda não foram medidos com o programa (item (c)
de `../docs/medicoes/F0.md`, que o gate da F3 fecha): se a extensão disser que falta o programa com
ele instalado, o suspeito é o confinamento do Snap, e o navegador do pacote `.deb` é o contorno.
