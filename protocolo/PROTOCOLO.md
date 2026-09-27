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
   chave a mais e até 64 KiB (medidos na cópia JSON que ele faz antes de ler). Todo o resto passa
   reto. Cada pedido vai ao fundo por uma PORTA própria, que cai quando a página fecha ou navega (e
   o fundo então encerra o fluxo e fecha a janela aberta). A resposta vai com `targetOrigin` igual à
   origem da página. O prazo da página é cobrado ali mesmo (`ola` 1,4 s, `listar` e `diagnostico`
   29,6 s, `assinar` 238 s): vencido, `tempo-esgotado`; extensão atualizada com a página aberta,
   `interno` mandando recarregar.
2. O fundo confere o REMETENTE (esta extensão, quadro 0, uma aba) e a ORIGEM que o navegador diz
   dele (`sender.origin`; no Firefox, a de `sender.url`), que precisa ser a declarada e estar nos
   padrões de algum emissor (`localhost` só no build de desenvolvimento). Senão, `origem-recusada`.
3. Tudo que não é `ola` exige a PERMISSÃO da pessoa para aquela origem: a janela `permitir.html`
   pergunta na primeira vez, e a decisão fica em `storage.local` (nunca sincronizada; uma chave por
   origem), revogável nas opções. Negada ou fechada, `permissao-negada`; sem resposta no prazo,
   `tempo-esgotado` (nada foi negado nem gravado).
4. `assinar` é `conferir` no programa, a janela `confirmar.html` (com o que o PROGRAMA leu do
   bilhete e do certificado e o endereço que o NAVEGADOR diz; o PIN só quando o dispositivo o
   exige) e `assinar` na mesma porta. Janela fechada ou "Cancelar" é `cancelado`; sem decisão,
   `tempo-esgotado`; token que o `conferir` diz bloqueado é `token-bloqueado`, sem janela; uma
   assinatura por vez (a segunda é `ocupado`). O programa que cai com a janela aberta fecha a
   janela na hora (`modulo-falhou` ou `nativo-ausente`).

Contra a página que abusa (um script numa página permitida): o `ola` é um só em voo e vale por
3 s; `listar` e `diagnostico` passam por uma fila do dispositivo (um por vez, até 4 esperando, o
resto é `ocupado`); e três janelas recusadas seguidas em 10 minutos embargam o endereço (10 minutos
para a de permissão, 2 para a de confirmação), com a recusa dizendo `embargo` no detalhe. No
Firefox, o `diagnostico` pedido por uma página exige o consentimento opcional de dado técnico
(`technicalAndInteraction`, na instalação ou nas opções); sem ele, `permissao-negada`.

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
| `modulo-falhou` | o módulo não carregou, falhou ou caiu (o filho morreu) no `assinar`; no `listar`, o módulo que falha ou passa do prazo (20 s) vira aviso, e os outros seguem |
| `interno` | a assinatura devolvida não confere com o certificado; falha inesperada |

No Windows, os mesmos códigos saem da recusa do provedor (os códigos do Windows vão no `detalhe`,
com a etapa):

| Código | Quando, no Windows |
|---|---|
| `cancelado` | `SCARD_W_CANCELLED_BY_USER`, `SCARD_E_CANCELLED`, `NTE_USER_CANCELLED`, `ERROR_CANCELLED` (o diálogo do provedor fechado), ou o canal com a extensão fechado durante a assinatura |
| `pin-incorreto` | `SCARD_W_WRONG_CHV`, sempre sem `tentativas` (o Windows não as informa) |
| `token-bloqueado` | `SCARD_W_CHV_BLOCKED` |
| `algoritmo-nao-suportado` | `NTE_BAD_ALGID`, `NTE_NOT_SUPPORTED` (o CSP legado sem SHA-256) |
| `certificado-nao-encontrado` | `SCARD_E_NO_SMARTCARD`, `SCARD_W_REMOVED_CARD` (o certificado continua no repositório, e o cartão dele não está na leitora: o "cartão removido" do Linux); o certificado que saiu do repositório entre a lista e a assinatura |
| `chave-ausente` | `NTE_NO_KEY`, `NTE_BAD_KEYSET`, `CRYPT_E_NO_KEY_PROPERTY`, e `NTE_BAD_PUBLIC_KEY` (a chave não é a do certificado) |
| `modulo-falhou` | `NTE_KEYSET_NOT_DEF` (o provedor do fabricante não está instalado), `NTE_PERM` e `ERROR_ACCESS_DENIED` (o provedor recusou o acesso), a recusa sem código (`E_FAIL`) e qualquer outra recusa do provedor |

`nativo-ausente` e `permissao-negada` são da extensão; o programa não os produz. (A
`permissao-negada` é a do ENDEREÇO, recusado na janela de permissão, ou, no Firefox, o diagnóstico
sem o consentimento do envio às páginas: um acesso negado pelo provedor do Windows é `modulo-falhou`,
e não ela.) `bilhete-expirado` e `nativo-desatualizado` estão no vocabulário e hoje nenhuma ponta os
produz: o programa responde `relogio` ao bilhete fora do prazo, e a versão mínima quem confere é a
biblioteca, no `ola`, com o `outdated` dela, que diz a peça (`nativo` ou `extensao`).
O que cada código significa para quem atende o chamado está em `docs/SUPORTE.md`.

## Módulos PKCS#11 (Linux e macOS)

Descoberta, em ordem, sem repetir o mesmo arquivo (caminho real):

1. o catálogo MEDIDO (`nativo/internal/catalogo`);
2. os registros do p11-kit (`*.module`), com as regras do `pkcs11.conf(5)`: o de mesmo nome se
   junta campo a campo, com o de `/etc/pkcs11/modules` sobre o de `/usr/share/p11-kit/modules` e o
   da pessoa (`~/.config/pkcs11/modules`) sobre os dois; a pasta da pessoa só conta se o
   `user-config` de `/etc/pkcs11/pkcs11.conf` não for `none` (com `only`, só ela); `module:` em
   branco desliga; o módulo por nome, sem caminho, se resolve no `$(libdir)/pkcs11` do Debian, do
   Ubuntu e do Fedora; `enable-in` sem `assinador` e `disable-in` com ele tiram o registro; o
   `p11-kit-trust` (repositório de ACs) e o `gnome-keyring` (senhas) ficam de fora;
3. `/etc/confidata-assinador/modulos.d/*.conf` e `~/.config/confidata-assinador/modulos` (um
   caminho absoluto por linha; `#` comenta).

O que foi pedido e não existe (do catálogo, registrado no p11-kit sem o arquivo, ou da
configuração) vai ao diagnóstico como `ausente`. O módulo achado pelo p11-kit ou pela configuração
com o NOME de arquivo de um do catálogo é aquele módulo (o rótulo e o genérico do catálogo): o
OpenSC registrado fora do caminho medido continua genérico na fusão, e o SafeSign num caminho que o
catálogo não mediu continua sendo o SafeSign.

Cada módulo roda num processo filho (`assinador modulo --caminho <x>`), que o próprio programa
lança, por operação. O filho lê o pedido no descritor 3 e responde no 4, com o mesmo quadro; a
entrada e a saída padrão dele apontam para `/dev/null`, porque biblioteca de fabricante que imprime
na tela corromperia o canal. O PIN vai num segundo quadro, em bytes, e o `C_Login` é um invólucro
próprio que copia o PIN para memória do C e a zera antes de liberar. Biblioteca que derruba o
processo derruba só o filho; a lista segue com os outros módulos e um aviso. Prazo de cada filho:
20 s no `listar`, 90 s no `assinar`.

Certificados iguais vistos por dois módulos (o SafeSign e o OpenSC no mesmo cartão) são fundidos
por `ref`, e fica o do fabricante.

## Chaves no Windows (CNG e CSP)

No Windows não há PKCS#11: o programa usa a API do próprio sistema, sem cgo
(`nativo/internal/windows`).

- **Lista:** o repositório pessoal do usuário (`CurrentUser\My`), cada certificado com
  `CERT_KEY_PROV_INFO_PROP_ID`, lido SEM abrir a chave (listar nunca pede PIN). O certificado do
  cartão chega ali pelo serviço de Propagação de Certificados; o A1 importado no Windows também
  aparece, e assina. `provedor` é `windows:cng` (a chave registrada num KSP) ou `windows:csp` (num
  CSP legado): é o registro, e na assinatura o CNG pode abrir por um KSP a chave registrada num CSP;
  `rotuloDoProvedor` é "Certificado instalado no Windows" para os provedores da Microsoft que guardam
  a chave no computador (o de software, o do TPM e o do Windows Hello), e o nome do provedor para o
  cartão ou token. `exigePin` é sempre falso: o PIN é
  pedido pelo PROVEDOR, num diálogo do Windows, e a janela da extensão não mostra campo.
- **Assinar:** o certificado é achado de novo no repositório pelo DER; a chave, por
  `CryptAcquireCertificatePrivateKey` com `CRYPT_ACQUIRE_PREFER_NCRYPT_KEY_FLAG` e
  `CRYPT_ACQUIRE_COMPARE_KEY_FLAG` (a chave tem de ser a do certificado). Chave CNG:
  `NCryptSignHash` com `BCRYPT_PAD_PKCS1` e SHA-256 sobre o resumo (o CNG monta o DigestInfo). Chave
  de CSP legado: `CryptCreateHash(CALG_SHA_256)`, `CryptSetHashParam(HP_HASHVAL)`,
  `CryptSignHash` e a inversão dos bytes (o CSP devolve em little-endian). O host confere a
  assinatura contra o certificado antes de ela sair, como no Linux.
- **Janela-mãe:** a janela que o programa dá ao provedor vai ao CSP (`PP_CLIENT_HWND`, antes de
  adquirir a chave), à aquisição (`CRYPT_ACQUIRE_WINDOW_HANDLE_FLAG`) e à chave CNG
  (`NCRYPT_WINDOW_HANDLE_PROPERTY`), para o diálogo de PIN abrir na frente do navegador. É o
  `--parent-window` do Chrome e do Edge quando ele não é zero; o Chrome documenta zero quando quem
  conecta é um contexto de fundo, que no Manifest V3 é o service worker da extensão, e o Firefox não
  passa janela. Com zero, é a janela em primeiro plano na hora de assinar (a confirmação da extensão
  onde a pessoa acabou de clicar). Se o diálogo e a janela ficam como devem é a medição (e) da F0.
- **Prazo:** a chamada ao provedor bloqueia enquanto o diálogo está aberto e não se interrompe; ela
  corre à parte, e quando a extensão fecha o canal (cada fluxo tem a sua conexão), o programa
  reabilita a janela-mãe (o diálogo modal a tinha desabilitado, e ela é do navegador), responde
  `cancelado` e sai, levando o diálogo junto. Não há o prazo de 90 s do filho do PKCS#11.
- **PIN errado:** quem pede o PIN é o diálogo do provedor, e há provedor que o pede de novo depois de
  um erro, sem devolver a recusa ao programa: a regra de encerrar no primeiro PIN errado (regra 2 do
  CLAUDE.md) só vale quando o provedor devolve `SCARD_W_WRONG_CHV`. O que cada provedor faz é medição.
- **Processo:** o CSP e o KSP do fabricante são DLLs que rodam DENTRO do programa (não há filho, como
  o plano decidiu para o Windows); a proteção do canal contra o que elas escrevem está em
  "Diagnóstico".

## Diagnóstico

A operação `diagnostico` e o modo `assinador diagnostico` do terminal (`--json` para o relatório)
devolvem o MESMO relatório (`nativo/internal/diagnostico`), para o suporte, sem CPF:

```
{ programa: { versao, protocolo, plataforma }, sistema,
  pcsc: { estado: 'ok' | 'sem-biblioteca' | 'sem-servico' | 'sem-leitora' | 'falhou', detalhe? },
  leitoras: [{ nome, comCartao, mudo?, atr?, cartao?, sugestao? }],
  provedores: [{ nome, caminho?, origem?, fabricante?, estado: 'carregado' | 'ausente' | 'falhou', certificados, detalhe? }],
  certificados: [{ titular, emissor?, validoAte?, situacao, provedor, leitor? }],
  avisos: [frase] }
```

- **PC/SC:** só estado. O programa lê as leitoras e o ATR de cada cartão (`SCardGetStatusChange`
  com prazo zero), nunca conecta ao cartão e nunca manda comando a ele. A biblioteca (o pcsc-lite,
  no Linux) é aberta com `dlopen` na hora da consulta: sem ela, o programa abre do mesmo jeito, e o
  diagnóstico diz o que instalar. No Windows é o `winscard.dll` do sistema, pelas funções `W`. As
  leitoras são consultadas ao mesmo tempo que os módulos; passados 5 s sem resposta do `pcscd` (no
  Windows, do serviço Cartão Inteligente), o relatório diz que ele não respondeu. A lista de
  leitoras que cresce entre o pedido do tamanho e o da lista é lida de novo.
- **Windows:** `sistema` vem do registro (o "Windows 10" que o registro ainda diz no Windows 11 é
  corrigido pela compilação); cada provedor de chave com certificado no repositório é um item de
  `provedores` (o nome do provedor, `detalhe` `cng` ou `csp`), e sem nenhum aparece o próprio
  repositório. O serviço Cartão Inteligente só roda com leitora conectada, então "sem serviço" ali
  vira "nenhuma leitora, ou o serviço parado". O estado do serviço de Propagação de Certificados
  (`CertPropSvc`, só leitura) entra como aviso quando há cartão lido, nenhum certificado de CARTÃO na
  lista (o instalado no computador não conta) e o serviço parado: é o caso mais provável de "não
  aparece", e o aviso dele toma o lugar do de instalar o programa do fabricante (sem cartão, o serviço
  parado é normal: ele inicia por gatilho).
  Pelo ATR do catálogo, a sugestão vale, mas o "não está instalado" não se afirma no Windows até o
  provedor do fabricante ser medido lá.
- **Sugestão pelo ATR:** o ATR que está no catálogo medido diz qual programa do fabricante lê o
  cartão (`cartao` e `sugestao`); sem esse programa carregado, o aviso manda instalá-lo. O módulo do
  catálogo se reconhece pelo rótulo ou pelo fabricante que declara no `C_GetInfo` (`fabricante`),
  e o "não achou certificado" se decide pela contagem do PRÓPRIO módulo.
- **Certificados:** o titular é o CN com os dígitos trocados por `*` (o CN ICP-Brasil é
  `NOME:CPF`); o emissor sai com as sequências de 11 ou mais dígitos mascaradas, e no autoassinado
  ele é o próprio titular. O de AC não entra, nem na lista nem na contagem do módulo. `situacao`:
  `valido`, `vencido`, `ainda-nao-valido`, `chave-nao-rsa`, `sem-uso-de-assinatura` ou `ilegivel`.
- **Texto de fora:** o que vem do aparelho e do certificado (o nome da leitora, o CN, o que o
  módulo declara) sai sem caractere de controle nem de direção, e com teto: um aparelho malicioso
  não forja linha no relatório nem manda sequência de escape ao terminal do suporte.
- **Avisos:** frases em português para a pessoa (a biblioteca do PC/SC falta; o `pcscd` não está
  rodando; nenhuma leitora; nenhuma leitora com cartão; o cartão não responde; o cartão usa um
  programa que não está instalado; o programa instalado não achou certificado; o cartão não foi lido
  por programa nenhum; um programa de cartão falhou; nenhum programa de cartão; certificado
  vencido). O `texto` é o relatório em frases, uma linha por item.

O host protege o canal do que o C escreve: no modo host, o descritor 1 aponta para `/dev/null`
desde o início, e só o canal com a extensão usa a cópia do descritor verdadeiro (que os filhos não
herdam). No Windows, onde o CSP e o KSP do fabricante rodam dentro do programa, são três camadas: o
`os.Stdout` do Go e a saída padrão do processo (`SetStdHandle`, que o C runtime de toda DLL
carregada depois lê ao iniciar) apontam para `NUL`, e o descritor 1 dos C runtimes compartilhados
que já estavam carregados (`msvcrt.dll`, `ucrtbase.dll`) é trocado por `_dup2`; o canal usa uma
cópia não herdável do handle verdadeiro.

## Pacotes para Linux

`instaladores/linux/empacotar.sh` monta os pacotes de DESENVOLVIMENTO (o programa com a tag `dev`
e os manifestos com o ID provisório): o `.deb` da arquitetura da máquina e, no amd64, o `.rpm`. O
programa vai para `/usr/lib/confidata-assinador/assinador`; o manifesto do Chrome, do Chromium e do
Edge para `/etc/opt/chrome`, `/etc/chromium` e `/etc/opt/edge` (`native-messaging-hosts/`), e o do
Firefox para `/usr/lib/mozilla` (e `/usr/lib64/mozilla` no `.rpm`). Depende da glibc 2.34 (o
empacotamento reprova se o binário passar a exigir mais) e recomenda o `pcscd` e o `libccid` (no
Fedora, `pcsc-lite` e `pcsc-lite-ccid`). `instaladores/linux/testar-pacotes.sh` prova os dois em
contêiner: fotografa `/etc`, `/usr` e `/opt`, instala, confere os manifestos (o ID e o caminho) e o
programa, remove, e reprova se a foto não voltar a ser a mesma. Os pacotes de produção, assinados e
com os IDs das lojas, são da F7a.
