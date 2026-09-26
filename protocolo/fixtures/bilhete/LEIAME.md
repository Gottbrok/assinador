# Fixtures do bilhete do Assinador

Geradas por `npm run fixtures:bilhete` a partir de `tests/apoio/casosDoBilhete.ts`.
Não edite à mão: mude a tabela e gere de novo. A assinatura ECDSA é aleatória,
então gerar de novo muda só a terceira parte de cada JWS.

São o contrato entre a referência em TypeScript (`src/bilhete.ts`) e o programa
nativo em Go (`Gottbrok/assinador`, `nativo/internal/bilhete/`), que as copia
para `protocolo/fixtures/bilhete/` com a tag e o SHA-256 de cada arquivo.

## Arquivos

- `protocolo.json`: as constantes (tipo, algoritmo, validade, tolerância, teto,
  limites de `doc` e `org`, padrões de origem por emissor, padrões de `dev`,
  faixas de controle e de espaço, formatos de `aud`, `sid` e dos resumos, o
  inteiro máximo, a ordem da conferência, o código de cada etapa e a lista de
  códigos de erro do protocolo). O Go copia DAQUI, e não do código TypeScript.
- `chaves.json`: as partes PÚBLICAS das chaves de teste, na forma de
  `protocolo/chaves-publicas.json` (`kid`, `iss`, `ambiente`, `jwk`).
- `casos/<nome>.json`: `{ nome, descricao, jws, entrada, esperado }`, com
  `entrada = { origem, digest, certificadoSha256, agora }` (`agora` em segundos
  UTC) e `esperado` igual a `{ ok: true, bilhete, chave }` ou
  `{ ok: false, codigo, etapa }`.

## Como conferir

Para cada caso: conferir `jws` com TODAS as chaves de `chaves.json`, a `origem`,
o `digest`, o `certificadoSha256` e o instante `agora`, e comparar com
`esperado`, inclusive a `etapa`. A etapa é o que prova a ORDEM: há casos com dois
defeitos (`ordem-*`), e vale o da etapa anterior.

## O que os casos obrigam no Go

Cada regra abaixo tem um caso que a reprova quando falta (entre parênteses).

**Forma e base64**
- Antes de decodificar qualquer parte, conferir que as TRÊS partes só têm
  `[A-Za-z0-9_-]`. O `base64.RawURLEncoding.Strict()` sozinho NÃO basta: ele
  pula `\r` e `\n` em silêncio (`cabecalho-com-quebra-de-linha`,
  `assinatura-com-quebra-de-linha`), e o `=` tem de falhar na forma, não na
  assinatura (`assinatura-com-preenchimento`).
- Decodificar com `RawURLEncoding.Strict()`: o cabeçalho na forma, a assinatura
  na etapa `assinatura`, a carga na etapa `carga` (`cabecalho-nao-canonico`,
  `assinatura-nao-canonica`, `carga-nao-canonica`).

**Cabeçalho**
- Decodificar num `map[string]any`, nunca numa struct: as chaves são
  comparadas EXATAMENTE (o `encoding/json` casa campo de struct sem diferenciar
  maiúscula, `cabecalho-com-chave-maiuscula`), só `alg`, `typ` e `kid` podem
  aparecer (`cabecalho-com-campo-extra`), e valor que não é texto é recusa em
  `alg` ou `kid`, não erro de decodificação (`alg-numerico`, `kid-numerico`).
- Número que não cabe em `float64` faz o `json.Unmarshal` recusar o documento:
  isso é `forma` (`cabecalho-com-numero-fora-do-float64`).
- O `kid` é comparado byte a byte com os pinados, sem outra conferência de
  formato (`kid-com-maiuscula`).

**Carga**
- `utf8.Valid` antes do `json.Unmarshal` (`carga-utf8-invalido`): o
  `encoding/json` troca o byte inválido por U+FFFD em silêncio.
- Decodificar num `map[string]any` com o decodificador padrão (números em
  `float64`, sem `UseNumber`). Número fora do `float64` recusa o documento na
  etapa `carga`, ANTES de `v` (`numero-fora-do-float64`); `v` vem antes dos
  campos (`ordem-v-antes-da-carga`, `ordem-v-antes-de-iat-nao-inteiro`, que uma
  struct tipada erraria), e `v` como texto é recusa em `v` (`v-como-texto`).
- `iat` e `exp` inteiros, de 0 a `inteiroMaximo` (`iat-nao-inteiro`,
  `iat-negativo`, `iat-acima-do-inteiro-seguro`).
- `aud`, `sid` e os resumos pelos `formatosDaCarga` (`aud-longa-demais`,
  `aud-nao-ascii`, `sid-longo-demais`, `sid-fora-do-formato`, `dig-maiusculo`).
- Limites de `doc` e `org` em PONTOS DE CÓDIGO (`utf8.RuneCountInString`,
  `texto-unicode-no-limite`, `doc-acima-do-limite`), e, sem os pontos das
  `faixasDeControle`, eles não podem ficar só com os de `faixasDeEspaco`
  (`doc-so-com-controles`, `org-so-com-espacos`).

**Saída e o resto**
- Na saída, `doc` e `org` sem os pontos das `faixasDeControle`, e surrogate
  solto vira U+FFFD, que o `encoding/json` já faz (`doc-e-org-com-controles`,
  `doc-com-surrogate-solto`).
- Tempo com as duas bordas INCLUSIVAS: `iat - tolerancia <= agora <= exp + tolerancia`
  (`limite-inferior-do-tempo`, `limite-superior-do-tempo`).
- Padrões de `dev` só para chave de ambiente `dev`, e ACRESCENTADOS aos do
  emissor (`valido-dev-localhost`, `localhost-com-chave-que-nao-e-dev`).

Os arquivos escrevem como `\uXXXX` todo caractere invisível ou ambíguo (C1,
controles bidirecionais, separadores, BOM, U+FFFD): texto que o editor mostra
diferente do que está no arquivo é o padrão "Trojan Source".

## Chaves de teste

As privadas são derivadas de semente pública (ver `tests/apoio/chavesDoBilhete.ts`):
qualquer um as recalcula. 🚫 Nenhuma delas entra em build que não seja de teste.

⚠️ `chaves.json` tem a MESMA forma do `~/.config/confidata-assinador/chaves-dev.json`
que o build `dev` do programa lê. Copiar este arquivo para lá faria o programa
de desenvolvimento aceitar bilhete forjado por qualquer pessoa, para `localhost`
e para `https://*.confidata.app`. Por isso o carregador das chaves `dev` do
programa TEM de recusar todo `kid` que comece por `teste` e toda JWK que esteja
neste arquivo, com teste.
