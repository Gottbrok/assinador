# Protocolo do Assinador

O vocabulário (versão, operações, códigos de erro, limites e o formato do bilhete) nasce na
biblioteca `@confidata/icp-brasil` (`src/browser/assinadorProtocolo.ts` e `src/bilhete.ts`) e é
espelhado aqui. As fixtures de `fixtures/bilhete/` são o contrato entre as duas pontas: a origem
delas está em `fixtures/ORIGEM.md`, e mudar o protocolo é mudar a biblioteca primeiro.

## As três pontas

```
página  --postMessage-->  extensão  --native messaging-->  programa  --PKCS#11 (filho)-->  cartão
```

A página nunca fala com o programa. A extensão confere a página, pede a permissão por endereço,
mostra a janela de confirmação e repassa ao programa. O programa confere o bilhete, lê os
certificados e assina um resumo.

## Página e extensão

`window.postMessage`, mesma janela, mesma origem:

```
pedido:   { canal: 'assinador:pedido',   v: 1, id, op: 'ola' | 'listar' | 'assinar' | 'diagnostico', dados? }
resposta: { canal: 'assinador:resposta', v: 1, id, ok: true, dados }
        | { canal: 'assinador:resposta', v: 1, id, ok: false, erro: { codigo, detalhe? } }
```

Prazos na página: `ola` 1,5 s; `listar` e `diagnostico` 30 s; `assinar` 240 s. Teto da mensagem:
64 KiB.

O que a extensão (`extensao/`) faz com cada pedido, nesta ordem:

1. O script de conteúdo, no mundo isolado e só no quadro de topo, aceita só a mensagem da própria
   janela (`event.source === window`), da origem da página, com `canal`, `v`, `id` com forma, sem
   chave a mais e até 64 KiB. Todo o resto passa reto. A resposta vai com `targetOrigin` igual à
   origem da página.
2. O fundo confere o REMETENTE (esta extensão, quadro 0, uma aba) e a ORIGEM que o navegador diz
   dele (`sender.origin`; no Firefox, a de `sender.url`), que precisa ser a declarada e estar nos
   padrões de algum emissor (`localhost` só no build de desenvolvimento). Senão, `origem-recusada`.
3. Tudo que não é `ola` exige a PERMISSÃO da pessoa para aquela origem: a janela `permitir.html`
   pergunta na primeira vez, e a decisão fica em `storage.local` (nunca sincronizada), revogável nas
   opções. Negada, fechada ou vencida, `permissao-negada`.
4. `assinar` é `conferir` no programa, a janela `confirmar.html` (com o que o PROGRAMA leu do
   bilhete e do certificado e o endereço que o NAVEGADOR diz; o PIN só quando o dispositivo o
   exige) e `assinar` na mesma porta. Janela fechada ou "Cancelar" é `cancelado`; sem decisão,
   `tempo-esgotado`; token que o `conferir` diz bloqueado é `token-bloqueado`, sem janela; uma
   assinatura por vez (a segunda é `ocupado`).

Cada operação tem um ORÇAMENTO dentro da extensão, abaixo do prazo da página (`ola` 1,2 s,
`listar` e `diagnostico` 29 s, `assinar` 235 s), e cada passo usa o menor entre o teto dele e o que
resta: a página recebe o `tempo-esgotado` da extensão e nunca desiste com uma janela ainda aberta. A
janela de confirmação fecha com 30 s de reserva para o cartão assinar; a de permissão, com 3 s.

## Extensão e programa (native messaging)

Cada mensagem é um quadro: 4 bytes com o tamanho do corpo, na ordem de bytes NATIVA da máquina, e
o corpo em JSON UTF-8. Teto: 64 KiB na entrada do programa, 1 MiB na saída (o limite do Chrome).
Mensagem acima do teto encerra o programa (o fluxo não se ressincroniza).

```
pedido:   { "v": 1, "id": "<id>", "op": "<operação>", "origem": "https://demot.confidata.app", "dados"?: { ... } }
resposta: { "v": 1, "id": "<id>", "ok": true, "dados": { ... } }
        | { "v": 1, "id": "<id>", "ok": false, "erro": { "codigo": "<código>", "detalhe"?: ... } }
```

**JSON estrito.** O programa lê o pedido com um leitor próprio (`nativo/internal/mensagens`), que
recusa como `protocolo`: chave desconhecida, chave repetida, chave em maiúscula, tipo errado, UTF-8
inválido, surrogate solto, caractere de controle cru, lista, `true`, `false`, `null`, objeto mais
fundo que `dados`, e conteúdo depois do objeto. `v` é exatamente o número `1`. `id` é
`^[A-Za-z0-9_-]{1,64}$`, e a recusa leva o `id` quando ele pôde ser lido (senão, `""`). `origem` é
`^https?://[a-z0-9.-]+(:[0-9]{1,5})?$`, até 256 caracteres.

**Quem chama.** O programa lê dos argumentos a extensão que o navegador diz ter lançado
(`chrome-extension://<id>/` no Chrome e no Edge, mais `--parent-window=<n>` no Windows; o caminho
do manifesto e o ID no Firefox). Fora das nossas extensões, toda operação responde
`origem-recusada` com o detalhe `chamador`. O ID do Firefox é fixo (`assinador@confidata.com.br`);
os do Chrome e do Edge das lojas entram na F7a. O build de desenvolvimento aceita também o ID que
o Chrome e o Edge derivam da chave pública de `extensao-dev.json` (provisório: a F3 o troca pelo do
rascunho do item na loja). A lista que o programa aceita é a MESMA que vai ao `allowed_origins`
dos manifestos (`nativo/cmd/manifestos`).

**Uma operação por vez.** Pedido que chega com outra em curso recebe `ocupado`. O programa sai
quando a entrada fecha, e os filhos dos módulos morrem com ele.

### Operações

| Operação | `dados` do pedido | `dados` da resposta |
|---|---|---|
| `ola` | nenhum | `{ versao, protocolo: 1, plataforma }` (a extensão acrescenta a versão dela) |
| `listar` | nenhum | `{ certificados: [{ ref, der, provedor, rotuloDoProvedor, leitor?, exigePin, estadoDoPin? }], avisos: [texto] }` |
| `conferir` | `{ ref, digest, bilhete }` | `{ emissor, organizacao, documento, finalidade, expiraEm, certificado: { assunto, emissor, validoAte, exigePin, estadoDoPin? } }` |
| `assinar` | `{ ref, digest, bilhete, pin? }` | `{ assinatura }` |
| `diagnostico` | nenhum | `{ relatorio, texto }` |

- `ref`: SHA-256 do DER do certificado, hexadecimal minúsculo. `digest`: o resumo SHA-256 a
  assinar, hexadecimal minúsculo de 64 caracteres. `der` e `assinatura`: base64 padrão.
- `assinatura`: RSA PKCS#1 v1.5 sobre o DigestInfo SHA-256 do `digest`. O programa a confere
  contra a chave pública do certificado antes de devolver.
- `listar` e `diagnostico` só atendem origem dos padrões de algum emissor (e `localhost` no build
  de desenvolvimento). `ola` responde a qualquer origem, porque só diz versões. A exceção é o
  `diagnostico` com a origem `https://extensao.invalid` (`origem.DaExtensao`), que a extensão informa
  só para pedido da página de OPÇÕES dela, conferida pelo remetente: o relatório não tem CPF, e o
  suporte o pede antes de a pessoa ter qualquer página autorizada. O `listar` nunca a aceita.
- `versao` do `ola` é `X.Y.Z` (a forma que a biblioteca compara com a versão mínima). O build de
  desenvolvimento informa a versão da PRÓXIMA publicação, e o sufixo de desenvolvimento fica só no
  nome do pacote (`1.0.0~dev.N`).
- `conferir` devolve o que a janela de confirmação mostra: o `doc`, a `org`, a `fin` e o `exp` do
  bilhete (este em RFC 3339, UTC), e o certificado escolhido. O `assunto` sai com os dígitos
  trocados por `*`: o CN ICP-Brasil é `NOME:CPF`, e o programa não interpreta campo ICP-Brasil (a
  leitura de nome, CPF e empresa é da biblioteca, no navegador). `exigePin` e `estadoDoPin` são os
  mesmos do `listar`, da mesma enumeração que achou o certificado: a janela mostra o campo de PIN só
  com `exigePin` e o aviso de tentativas pelo `estadoDoPin` (F3). A extensão não precisa listar de
  novo para saber.
- `assinar` confere tudo de novo: o programa não guarda estado entre `conferir` e `assinar`.
- `pin`: texto de até 64 bytes, sem caractere de controle. Só vai ao cartão quando o token exige
  PIN pelo `C_Login`; se o token tem caminho protegido de autenticação (teclado na leitora, ou
  diálogo do middleware), o `pin` que vier é ignorado. PIN ausente ou vazio num token que exige é
  `protocolo`, e nada vai ao cartão (há middleware que conta o vazio como tentativa errada).
- Certificado listado: tem `keyUsage` com `digitalSignature` ou `nonRepudiation`, não é de AC, e a
  chave é RSA. Certificado vencido é LISTADO e nunca assinado. Chave de outro tipo sai da lista com
  um aviso.
- `estadoDoPin`: `ok`, `poucas-tentativas`, `ultima-tentativa` ou `bloqueado`, das flags
  `CKF_USER_PIN_*` do token.

### Ordem do `conferir` e do `assinar`

1. O chamador é nosso, e a origem está nos padrões de algum emissor.
2. O bilhete, na ordem da seção seguinte, com `cer` igual à `ref`. Nenhum módulo é carregado
   antes disso: pedido forjado não toca biblioteca de fabricante.
3. O certificado de `ref` está num dispositivo presente, é listável e está dentro da validade.
4. (`assinar`) O PIN, se o token o exige; o login e a assinatura no filho do módulo; a
   assinatura conferida contra o certificado.

## O bilhete

JWS compacto ES256 emitido pelo servidor que preparou o resumo (`src/bilhete.ts` da biblioteca).
Cabeçalho `{ alg: "ES256", typ: "assinador+jws", kid }`; carga
`{ v: 1, iss, aud, sid, dig, cer, fin, doc, org, iat, exp }`, com `exp - iat = 300`; teto de 4 KiB.

A conferência para na primeira falha, nesta ordem, e cada etapa tem o seu código:

| Etapa | Código |
|---|---|
| forma, alg, typ, kid, assinatura, carga (objeto), v, carga (campos), iss | `bilhete-invalido` |
| aud (igual à origem informada), padrao (a origem nos padrões do emissor da chave) | `origem-recusada` |
| dig (igual ao `digest` pedido) | `digest-divergente` |
| cer (igual ao SHA-256 do DER de `ref`) | `certificado-divergente` |
| tempo: `iat - 900 <= agora <= exp + 900` | `relogio` |

O `detalhe` da recusa é `etapa <nome>`, para o suporte. As regras finas (base64url canônico,
cabeçalho decodificado em mapa, números em `float64`, limites de `doc` e `org` em pontos de
código, as faixas de controle e de espaço) estão no `LEIAME.md` das fixtures, cada uma com o caso
que a prova; `nativo/internal/bilhete` roda todos os casos.

**Padrões de origem por emissor:** `confidata` só `^https://[a-z0-9-]{1,63}\.confidata\.app$`;
`ushield` só `^https://ushield\.app$`. Chave de ambiente `dev` acrescenta
`^http://([a-z0-9-]+\.)?localhost(:[0-9]+)?$`.

### Chaves pinadas

- `chaves-publicas.json` lista as chaves PÚBLICAS de produção, `{ kid, iss, ambiente: "producao", jwk }`.
  `nativo/internal/bilhete/chaves.go` é GERADO dele (`go generate ./internal/bilhete`), e o gerador
  recusa qualquer outro ambiente. Hoje a lista é vazia: o programa de release não aceita bilhete
  nenhum até a F7a pinar as chaves de produção (falha fechada).
- As chaves de TESTE ficam só nas fixtures, e só os testes as usam. As privadas delas são públicas
  (sementes no `LEIAME.md`).
- O build com a tag `dev` lê também `~/.config/confidata-assinador/chaves-dev.json` (mesma forma),
  e recusa ali toda entrada que não seja de ambiente `dev`, todo kid que comece por `teste` e toda
  coordenada das chaves de teste.
- Rotação: gerar a chave nova INATIVA no console do emissor, publicar o programa que pina as duas,
  subir a `versaoMinima` e só então ativar a nova.

## Códigos de erro

`origem-recusada`, `bilhete-invalido`, `bilhete-expirado`, `relogio`, `digest-divergente`,
`certificado-divergente`, `certificado-nao-encontrado`, `chave-ausente`, `algoritmo-nao-suportado`,
`permissao-negada`, `pin-incorreto`, `token-bloqueado`, `cancelado`, `tempo-esgotado`, `ocupado`,
`nativo-ausente`, `nativo-desatualizado`, `modulo-falhou`, `protocolo`, `interno`.

O conjunto é o da biblioteca, e `nativo/internal/protocolo` o compara com a fixture. O `detalhe` é
texto para o suporte, exceto em `pin-incorreto`, onde é `{ tentativas: "poucas" | "ultima" }` (ou
ausente, quando o token não diz). Detalhe nunca leva PIN, CPF, nome de titular nem conteúdo de
pedido.

O que o programa produz, e quando:

| Código | Quando |
|---|---|
| `certificado-nao-encontrado` | `ref` em nenhum dispositivo presente; certificado vencido, ainda não válido ou sem uso de assinatura; cartão removido no meio |
| `chave-ausente` | depois do login, o token não tem a chave privada do certificado |
| `algoritmo-nao-suportado` | chave que não é RSA, ou mecanismo recusado pelo token |
| `pin-incorreto` | `CKR_PIN_INCORRECT`, `CKR_PIN_INVALID`, `CKR_PIN_LEN_RANGE` (as tentativas relidas das flags DEPOIS da falha) |
| `token-bloqueado` | `CKR_PIN_LOCKED`, `CKR_PIN_EXPIRED`, token já bloqueado antes do login, ou o PIN errado que bloqueou |
| `cancelado` | `CKR_FUNCTION_CANCELED` (a pessoa cancelou no leitor ou no diálogo do middleware) |
| `tempo-esgotado` | o filho do módulo não respondeu no prazo do `assinar` (90 s) |
| `modulo-falhou` | o módulo não carregou, falhou ou caiu (o filho morreu) |
| `interno` | a assinatura devolvida não confere com o certificado; falha inesperada |

`nativo-ausente`, `nativo-desatualizado`, `permissao-negada` e `bilhete-expirado` são da extensão ou
da biblioteca; o programa não os produz.

## Módulos PKCS#11 (Linux e macOS)

Descoberta, em ordem, sem repetir o mesmo arquivo (caminho real):

1. o catálogo MEDIDO (`nativo/internal/catalogo`);
2. os registros do p11-kit (`~/.config/pkcs11/modules`, `/etc/pkcs11/modules` e
   `/usr/share/p11-kit/modules`, `*.module`): o de mesmo nome numa pasta anterior esconde os das
   seguintes; o módulo por nome, sem caminho, se resolve no `$(libdir)/pkcs11` do Debian, do Ubuntu
   e do Fedora; `enable-in` sem `assinador` e `disable-in` com ele tiram o registro; o
   `p11-kit-trust` (repositório de ACs) e o `gnome-keyring` (senhas) ficam de fora;
3. `/etc/confidata-assinador/modulos.d/*.conf` e `~/.config/confidata-assinador/modulos` (um
   caminho absoluto por linha; `#` comenta).

O que foi pedido e não existe (do catálogo, registrado no p11-kit sem o arquivo, ou da
configuração) vai ao diagnóstico como `ausente`.

Cada módulo roda num processo filho (`assinador modulo --caminho <x>`), que o próprio programa
lança, por operação. O filho lê o pedido no descritor 3 e responde no 4, com o mesmo quadro; a
entrada e a saída padrão dele apontam para `/dev/null`, porque biblioteca de fabricante que imprime
na tela corromperia o canal. O PIN vai num segundo quadro, em bytes, e o `C_Login` é um invólucro
próprio que copia o PIN para memória do C e a zera antes de liberar. Biblioteca que derruba o
processo derruba só o filho; a lista segue com os outros módulos e um aviso. Prazo de cada filho:
20 s no `listar`, 90 s no `assinar`.

Certificados iguais vistos por dois módulos (o SafeSign e o OpenSC no mesmo cartão) são fundidos
por `ref`, e fica o do fabricante.

## Diagnóstico

A operação `diagnostico` e o modo `assinador diagnostico` do terminal (`--json` para o relatório)
devolvem o MESMO relatório (`nativo/internal/diagnostico`), para o suporte, sem CPF:

```
{ programa: { versao, protocolo, plataforma }, sistema,
  pcsc: { estado: 'ok' | 'sem-biblioteca' | 'sem-servico' | 'sem-leitora' | 'falhou', detalhe? },
  leitoras: [{ nome, comCartao, mudo?, atr?, cartao?, sugestao? }],
  provedores: [{ nome, caminho?, origem?, estado: 'carregado' | 'ausente' | 'falhou', certificados, detalhe? }],
  certificados: [{ titular, emissor?, validoAte?, situacao, provedor, leitor? }],
  avisos: [frase] }
```

- **PC/SC:** só estado. O programa lê as leitoras e o ATR de cada cartão (`SCardGetStatusChange`
  com prazo zero), nunca conecta ao cartão e nunca manda comando a ele. A biblioteca (o pcsc-lite,
  no Linux) é aberta com `dlopen` na hora da consulta: sem ela, o programa abre do mesmo jeito, e o
  diagnóstico diz o que instalar. Passados 5 s sem resposta do `pcscd`, o relatório diz que ele não
  respondeu.
- **Sugestão pelo ATR:** o ATR que está no catálogo medido diz qual programa do fabricante lê o
  cartão (`cartao` e `sugestao`); sem esse programa instalado, o aviso manda instalá-lo.
- **Certificados:** o titular é o CN com os dígitos trocados por `*` (o CN ICP-Brasil é
  `NOME:CPF`); o emissor sai como está, exceto no autoassinado, em que ele é o próprio titular. O de
  AC não entra. `situacao`: `valido`, `vencido`, `ainda-nao-valido`, `chave-nao-rsa`,
  `sem-uso-de-assinatura` ou `ilegivel`.
- **Avisos:** frases em português para a pessoa (a biblioteca do PC/SC falta; o `pcscd` não está
  rodando; nenhuma leitora; nenhuma leitora com cartão; o cartão não responde; o cartão usa um
  programa que não está instalado; o programa instalado não achou certificado; o cartão não foi lido
  por programa nenhum; um programa de cartão falhou; nenhum programa de cartão; certificado
  vencido). O `texto` é o relatório em frases, uma linha por item.

O host protege o canal do que o C escreve: no modo host, o descritor 1 aponta para `/dev/null`
desde o início, e só o canal com a extensão usa a cópia do descritor verdadeiro (que os filhos não
herdam).

## Pacotes para Linux

`instaladores/linux/empacotar.sh` monta os pacotes de DESENVOLVIMENTO (o programa com a tag `dev`
e os manifestos com o ID provisório): o `.deb` da arquitetura da máquina e, no amd64, o `.rpm`. O
programa vai para `/usr/lib/confidata-assinador/assinador`; o manifesto do Chrome, do Chromium e do
Edge para `/etc/opt/chrome`, `/etc/chromium` e `/etc/opt/edge` (`native-messaging-hosts/`), e o do
Firefox para `/usr/lib/mozilla` (e `/usr/lib64/mozilla` no `.rpm`). Recomenda o `pcscd` e o
`libccid` (no Fedora, `pcsc-lite` e `pcsc-lite-ccid`). `instaladores/linux/testar-pacotes.sh` prova
os dois em contêiner: instala, roda, remove, e nada sobra. Os pacotes de produção, assinados e com
os IDs das lojas, são da F7a.
