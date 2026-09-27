# Registro de mudanças

## Não publicado

- F0: `CLAUDE.md`, `README.md`, ferramenta de prova PKCS#11 (`ferramentas/prova`) e kit de medição do
  native messaging (`ferramentas/prova-mensagens`). Medições em `docs/medicoes/F0.md`.
- Auditoria da F0: PIN vazio nunca vai ao cartão (há middleware que conta como tentativa errada); a leitura
  do PIN pela entrada padrão não deixa cópia em buffer; a cópia do resultado na extensão tem recuo.
- F2a: o programa nativo (`nativo/`), no Linux. Modos `host` (native messaging) e `modulo` (filho
  PKCS#11); operações `ola`, `listar`, `conferir`, `assinar` e um `diagnostico` mínimo (a F2b o
  completa). Bilhete conferido contra as 78 fixtures da biblioteca (tag `v0.4.0`, idênticas às da
  `v0.3.0`), com o código e a etapa de cada uma; leitor JSON estrito próprio, para o PIN nunca virar
  `string`; `C_Login` por invólucro próprio que zera a cópia do PIN (achado da F0); um filho por
  módulo, com a saída padrão em `/dev/null`; fusão por `ref` preferindo o módulo do fabricante;
  assinatura conferida contra o certificado antes de sair. Catálogo com o SafeSign e o OpenSC
  medidos na F0. Testes com o SoftHSM2 (inclusive a assinatura conferida pelo `openssl`), um módulo
  de teste que aborta ou trava no `C_Initialize`, a ponta a ponta com o binário de desenvolvimento, e
  a catraca que confere os bytes do binário de release. CI `nativo.yml`. Ferramenta `host-teste`.
  Documentação: `protocolo/PROTOCOLO.md`, `SECURITY.md` e o `README.md`.
  - Divergências do plano, decididas na implementação: `protocolo/chaves-publicas.json` lista só as
    chaves de PRODUÇÃO (hoje nenhuma: o release não aceita bilhete até a F7a), e as de teste ficam só
    nas fixtures, só para os testes; o release aceita só a extensão do Firefox (os IDs do Chrome e do
    Edge entram na F7a, os de desenvolvimento na F3); o catálogo tem só os caminhos medidos do Ubuntu
    x86_64 (o Fedora e o arm64 entram quando medidos); num token que exige login, o certificado sem
    chave visível também é listado (a chave pode estar escondida até o PIN), e a AC sai pela regra
    de uso; o `assunto` do `conferir` sai com os dígitos mascarados; o programa aceita certificado com
    número de série negativo (`x509negativeserial=1`), porque só o lê.
- Auditoria da F2a (uma revisão adversarial independente e a nossa): um P1 corrigido, o filho do
  módulo responde ANTES de encerrar (logout, sessões, `C_Finalize` e `dlclose` de fabricante que
  travam não custam mais a resposta de um cartão que já assinou), e o logout é só do login que o
  próprio filho fez. Do lote de robustez que o Cairo escolheu: o SIGTERM encerra; filho morto com o
  canal herdado por um auxiliar da biblioteca não prende o pai até o prazo; o código de erro do
  filho é conferido contra o vocabulário; o `CKA_ID` só vale se a chave não desmentir o certificado;
  o host recalcula a `ref` do DER. `go.mod` sem o `// indirect` errado.
  - Ficaram para decisão (P2 e P3): um teste que CONTE os `C_Login` (hoje nada prova, por contagem,
    que o PIN errado não se repete: o SoftHSM nunca bloqueia) e o do fim da entrada no meio da
    operação; a checagem vazia do emissor em `TestConferirMostraOBilheteEOCertificadoSemCpf`, e o
    emissor de certificado AUTOASSINADO sai sem máscara no `conferir` (o de AC real não tem CPF);
    o chamador conferido antes da forma do bilhete (hoje a forma vem antes, sem vazar nada); o PIN
    aceita controles C1; o `PROTOCOLO.md` diz que o vocabulário da ponte extensão e programa nasce
    na biblioteca, e ele nasce aqui (regra 6); o `internal/softhsmteste` quebra o `go vet` do
    Windows (falta a tag); no `host-teste`, o erro de `ref` não encontrada imprime o DER, que tem o
    CPF (regra 9), e o PIN do terminal passa pelo `term.ReadPassword`, que deixa cópias.
  - Para medir com o cartão: certificado em mais de um slot do mesmo módulo e o SafeSign e o OpenSC
    listando o mesmo cartão ao mesmo tempo (`docs/medicoes/F2a.md`). Para a F3: os `avisos` do
    `listar` são frases em português, e a página ainda não os mostra; mostrá-los em espanhol pede
    código no lugar da frase, o que muda o protocolo na biblioteca.
  - Aceito por desenho: `CKR_USER_ALREADY_LOGGED_IN` (middleware que compartilha o login entre
    aplicações) segue sem conferir o PIN digitado, como nos outros assinadores.
- F2b: o diagnóstico, o catálogo por ATR, o p11-kit e os pacotes para Linux.
  - `pcsc`: as leitoras e o ATR de cada cartão pelo pcsc-lite, só estado (nunca conecta ao cartão),
    com a biblioteca aberta por `dlopen` na hora (o programa abre sem ela) e o desenho das
    estruturas conferido contra os cabeçalhos de verdade (tag `pcsc_cabecalho`, no CI).
  - `diagnostico`: o relatório e o texto do suporte, sem CPF (o titular mascarado; o emissor do
    autoassinado também), com a sugestão do programa do fabricante pelo ATR e os avisos em frase;
    responde à operação `diagnostico` e ao modo novo `assinador diagnostico [--json]` do terminal.
    O `pcscd` que não responde em 5 s não segura o relatório.
  - `catalogo`: o tipo `ATR` (o cartão medido e o módulo que o lê), ainda sem entrada: o ATR do
    cartão Certisign do Cairo entra no gate, lido pelo diagnóstico. O OpenSC ganhou o caminho do
    Fedora (`/usr/lib64/opensc-pkcs11.so`), medido num Fedora 43 em contêiner.
  - `pkcs11`: os registros do p11-kit como segunda fonte (o da pessoa sobre o de `/etc`, este
    sobre o do pacote; `enable-in` e `disable-in`; o `p11-kit-trust` e o `gnome-keyring` de fora; o
    módulo por nome resolvido no `$(libdir)/pkcs11`), e a origem de cada módulo no diagnóstico.
  - `main`: o descritor 1 do host aponta para `/dev/null` desde o início, e só o canal usa a cópia
    do descritor verdadeiro (com CLOEXEC): o que o C escreve (o pcsc-lite, agora no processo do
    host) não corrompe o quadro.
  - `origem` e `cmd/manifestos`: o ID provisório da extensão de desenvolvimento, derivado da chave
    PÚBLICA em `protocolo/extensao-dev.json` (a privada não existe; a F3 troca pela do rascunho da
    loja), aceito só no build `dev` (a catraca confere que o release não o contém); os manifestos
    dos navegadores saem da MESMA lista que o programa confere.
  - `instaladores/linux`: o `.deb` (amd64 e arm64) e o `.rpm` (x86_64) de DESENVOLVIMENTO pelo
    `nfpm`, e a prova em contêiner (instala, roda, remove, nada sobra); no CI, compilados nativos em
    cada arquitetura e guardados como artefato, sem assinatura até a F7a.
  - Divergências do plano: os pacotes da F2b levam o build `dev` (é o que o teste manual das F4b e
    F5 precisa, e o release não aceita bilhete nenhum até a F7a); o ATR do Cairo, que o plano punha
    no escopo, espera o gate, porque não havia leitora nesta máquina; os avisos e o texto do
    diagnóstico são frases em português, como os `avisos` do `listar` (a mesma decisão da F3); o
    PC/SC é aberto por `dlopen` próprio, e não pelo `github.com/ebfe/scard` que a §3.5 do plano
    nomeava (o `scard` liga a biblioteca na compilação, e o programa não abriria sem o pcsc-lite
    instalado); e o p11-kit lê também a pasta da pessoa (`~/.config/pkcs11/modules`), que o p11-kit
    lê, além das duas que o plano listava.
- Auditoria da F2b (uma revisão adversarial independente e a nossa): nenhum P0; um P1 corrigido, o
  CI vermelho no primeiro push (no executor do GitHub o SoftHSM vem registrado no p11-kit, e os
  testes de ponta a ponta esperavam o nome que ele tem só onde não está instalado no sistema). Os
  quatro lotes que o Cairo escolheu, todos corrigidos:
  - diagnóstico: a sugestão pelo ATR reconhece o módulo do catálogo pelo rótulo OU pelo fabricante
    que ele declara (antes, o SafeSign achado pela configuração virava "não está instalado"), e cada
    módulo conta os PRÓPRIOS certificados, menos os de AC; o texto de fora sai sem controle (o
    aparelho malicioso não forja linha nem escape no terminal); o emissor mascara 11 ou mais
    dígitos; a frase do PC/SC ausente não fala em pcscd fora do Linux; as leitoras são consultadas
    ao mesmo tempo que os módulos, e o cancelamento não espera o prazo do pcscd;
  - PC/SC e p11-kit: a lista de leitoras que cresce no meio é relida; o pcsc-lite FALSO dá teste ao
    caminho com leitora e cartão; o registro do p11-kit de mesmo nome se junta campo a campo, com o
    `user-config`, como no `pkcs11.conf(5)`; o módulo com o nome de arquivo de um do catálogo é
    aquele módulo (o OpenSC registrado continua genérico);
  - testes e docs: o teste do `gnome-keyring` prova a exclusão de verdade; o host de verdade prova
    o canal com o C escrevendo na saída padrão; o registro de medição tinha números que não foram
    medidos (64 bytes e deslocamento 28; o certo é 80 e 40);
  - pacotes: declaram a glibc 2.34 (e o empacotamento reprova se o binário exigir mais); o `.deb`
    devolve o `/etc/opt` que o `dpkg` apagava; o teste compara o sistema inteiro antes e depois.
- F3: a extensão (`extensao/`), Manifest V3 para Chrome, Edge e Firefox, TypeScript e Vite sem
  framework, um arquivo por script e sem minificar.
  - A ponte da página (mundo isolado, quadro de topo), o fundo com os quatro portões (remetente,
    origem dita pelo navegador, permissão por endereço, forma), a porta de native messaging (um
    pedido por vez; prazo vencido descarta a porta), as janelas `permitir.html` e `confirmar.html`
    (o que o programa leu do bilhete e o host que o navegador diz; PIN só quando o dispositivo o
    exige; botões travados por 600 ms depois de a janela ficar visível, contra clique cronometrado
    pela página) e as opções (endereços permitidos com "Remover", versões, diagnóstico).
  - O orçamento de prazos: cada operação tem um prazo dentro da extensão abaixo do da página, e cada
    passo usa o menor entre o teto dele e o que resta. Somados, os tetos passavam do prazo da página
    (permissão mais `listar`; `conferir`, janela e `assinar`), e a página desistiria com a janela
    ainda aberta.
  - Textos em `_locales/pt_BR` e `_locales/es`, com teste de paridade de chaves e marcadores.
    Ícones provisórios gerados por script. Pacotes da loja reproduzíveis (`npm run reproduzivel`),
    o do Chrome sem `key`. `web-ext lint` sem erro. CI `extensao.yml`, com o ponta a ponta no
    Chromium do Playwright, o programa de desenvolvimento e o SoftHSM2.
  - Programa: o `conferir` devolve `exigePin` e `estadoDoPin` do certificado (a janela precisa
    saber se mostra o campo do PIN); o `diagnostico` atende a origem reservada
    `https://extensao.invalid`, e só ele, porque as opções o pedem antes de haver endereço
    autorizado; a versão informada é sempre `X.Y.Z` (o sufixo `~dev.N` fica só no nome do pacote).
  - `host-teste servir`: a página de teste em `http://localhost`, com o bilhete emitido pela chave
    dev local e a assinatura conferida contra o certificado; é a mesma página do ponta a ponta e do
    teste manual com o cartão.
  - Divergências do plano: a extensão pede `storage` além de `nativeMessaging` (a lista de endereços
    permitidos, que o próprio plano exige, mora em `storage.local`); a página de teste fala o
    protocolo à mão, e não pelo bundle da biblioteca, porque este repositório é público e a
    biblioteca é privada; a chave de desenvolvimento segue a provisória até existir o rascunho do
    item na Chrome Web Store (ato do Cairo).
- Auditoria da F3 (uma revisão adversarial independente e a nossa): nenhum P0 nem P1; quatro P2 e
  nove P3, todos corrigidos.
  - Ponte: uma PORTA por pedido (`runtime.connect`), que cai quando a página sai, e o fundo então
    encerra o fluxo e fecha a janela; o `connect` que lança na hora (extensão atualizada com a
    página aberta) responde `interno` em vez de deixar a página esperar o prazo; o prazo da página é
    cobrado no script de conteúdo, no relógio dela (antes, o orçamento contava da chegada ao fundo,
    e um service worker frio fazia a página achar que a extensão não estava instalada); a mensagem
    da página é lida de uma cópia JSON.
  - Abuso: o `ola` é um só em voo e vale 3 s; `listar` e `diagnostico` passam pela fila do
    dispositivo (um por vez, até 4 esperando); três janelas recusadas seguidas embargam o endereço.
  - Janelas: a trava dos botões exige janela visível E com foco, trava de novo quando o foco sai e
    recomeça a contagem, e ignora tecla segurada; o foco vai para o controle seguro só depois de
    destravar (antes, focar botão desabilitado não fazia nada, e o plano mandava focar "Assinar");
    a janela de permissão que vence é `tempo-esgotado`, e não `permissao-negada`; o programa que
    cai com a janela aberta fecha a janela na hora.
  - Permissões numa chave por origem (antes, o fundo e as opções regravavam o mesmo mapa e podiam
    desfazer um ao outro); origem de remetente presente e opaca é recusa, sem recuar à URL.
  - Firefox: a declaração de dados passou de `none` para o que a extensão entrega à página (o
    certificado, com nome e CPF, obrigatório; o diagnóstico, opcional e com consentimento).
  - Reprodução: o carimbo do zip é o do último commit que tocou `extensao/` ou `protocolo/`, o build
    imprime o `SOURCE_DATE_EPOCH`, e a conferência muda fuso e `NODE_ENV` entre os dois builds.
  - Testes que não provavam o que diziam foram reescritos (o encurtamento da janela pelo orçamento,
    a permissão numa chave só); o exemplo de host nos textos da loja virou um neutro.
  - Fica como teste manual do gate: o Firefox (o ponta a ponta automático é só no Chromium).
- F6a: o programa no Windows, sem cgo e sem PKCS#11.
  - `windows`: o repositório pessoal do usuário pelo `CERT_KEY_PROV_INFO` (listar nunca abre a
    chave), a assinatura pelo CNG (`NCryptSignHash`, PKCS#1 com SHA-256) ou pelo CSP legado
    (`CryptSignHash` sobre o `HP_HASHVAL`, com a inversão de bytes), a chave conferida contra o
    certificado na aquisição (`COMPARE_KEY`), a janela-mãe do Chrome no CSP, na aquisição e na chave
    CNG, e o mapa dos códigos do Windows para o protocolo. O PIN é do provedor (`exigePin` falso). A
    chamada que bloqueia no diálogo corre à parte, e o canal fechado responde na hora.
  - `pcsc`: o `winscard.dll` pelas funções `W`. `diagnostico`: o nome do sistema pelo registro (o
    Windows 11 pela compilação), as frases do Windows, e o serviço de Propagação de Certificados
    (`CertPropSvc`), cujo aviso vale só com cartão lido e nenhum certificado na lista, porque ele
    inicia por gatilho e parado sem cartão é normal.
  - `main`: o canal separado da saída padrão também no Windows, onde o CSP e o KSP do fabricante
    rodam dentro do programa (a F6a decidiu que sim): o handle do processo em `NUL` e o descritor 1
    do `msvcrt` e do `ucrtbase` já carregados por `_dup2`.
  - Testes no CI (`windows-latest` e `windows-11-arm`): o `windowsteste` cria certificados com chave
    de software pelo `New-SelfSignedCertificate` (o papel do SoftHSM), o caminho CSP é forçado para
    provar a inversão, o canal é provado com os dois C runtimes escrevendo, e a ponta a ponta roda o
    programa `dev` lançado como o Chrome. As estruturas e os códigos são conferidos contra o SDK (o
    MinGW), medidos também aqui (`docs/medicoes/F6a.md`). O job do Linux compila o Windows cruzado.
  - `ferramentas/registrar-windows.ps1`: grava e remove o registro de desenvolvimento (a pasta do
    usuário, os manifestos gerados, as chaves `HKCU`), provado no CI.
  - A suíte diz o mesmo em qualquer executor: a descoberta PKCS#11 e o apoio do SoftHSM ficam fora
    do Windows, e os testes do diagnóstico fixam o sistema.
  - Divergências do plano, decididas na implementação: o Windows não tem o processo filho da regra
    8 (o provedor do fabricante roda dentro do programa, como o §3.5 descreve), e por isso o canal
    ganhou a proteção da regra 12; a assinatura no Windows não tem o prazo de 90 s do filho do
    PKCS#11 (o diálogo do provedor não se interrompe, e o canal fechado encerra); a medição (d) e (e)
    da F0 e o catálogo do Windows esperam a máquina Windows com o cartão; os testes rodam também no
    arm64 (`windows-11-arm`), que o plano deixava para a F6b; e a "dica no diagnóstico" do CSP sem
    SHA-256 (§3.5) mora no `detalhe` do `algoritmo-nao-suportado`, porque o diagnóstico não assina e
    não tem como saber que o CSP não tem SHA-256.
- Auditoria da F6a (uma revisão adversarial independente e a nossa), antes do primeiro CI do
  Windows: nenhum P0. Dois P1 corrigidos, que reprovariam esse CI por motivos alheios ao provedor: o
  checkout em CRLF do Git do Windows (o `.gitattributes` fixa LF, e as fixtures sem conversão), que
  quebrava o hash das fixtures e o `chaves.go` gerado; e o teste do catálogo, que conferia caminho
  Unix com o `filepath` do Windows. Os P2 e P3, corrigidos por decisão do Cairo:
  - o canal: o handle original protegido contra fechamento antes dos `_dup2` (com os dois C runtimes
    carregados, o segundo fechava de novo um número que o Windows já podia ter dado a outro
    recurso), o descritor 1 de cada C runtime conferido depois, e o teste carregando os dois C
    runtimes ANTES de separar o canal (antes, ele passava sem o `_dup2`);
  - a janela-mãe: o Chrome documenta `--parent-window` zero para o contexto de fundo (o service
    worker do Manifest V3), e o código da janela nunca rodaria; com zero, vale a janela em primeiro
    plano na hora de assinar, e no cancelamento ela é reabilitada;
  - o mapa de erros: acesso negado e o provedor que não existe viram `modulo-falhou` (a
    `permissao-negada` da biblioteca é a do endereço na extensão), a chave que não é a do
    certificado vira `chave-ausente`, o cartão fora da leitora vira `certificado-nao-encontrado`
    (como no Linux), e a recusa sem código vira `E_FAIL`, nunca o zero do sucesso;
  - o diagnóstico: os avisos do cartão contam só os certificados de CARTÃO (um A1 instalado
    silenciava o aviso do `CertPropSvc`), o `Montar` volta a ser puro, e o texto do Linux volta a
    dizer "serviço pcscd parado";
  - os testes: o `windowsteste` lê a última linha da saída padrão e limpa pelo assunto antes de ler
    (não vaza certificado), e o CSP sem SHA-256 é obrigatório no CI;
  - o registro e a documentação: caminhos literais, as chaves-mãe vazias removidas, a política de
    execução e o `Unblock-File` nas instruções, o Windows Hello como chave do computador, o
    `KeepAlive` do nome do repositório, a regra 2 no Windows e o que medir a mais com o cartão.
- O primeiro CI do Windows: o x64 verde inteiro, com os testes de cabeçalho rodando contra o MinGW
  do executor. No arm64, dois `New-SelfSignedCertificate` ao mesmo tempo (os pacotes rodam em
  paralelo) recusaram com "Access is denied", e a limpeza dos certificados de teste falhava em
  silêncio nos dois (o `-DeleteKey` pelo pipe não existe). A criação e a remoção passaram a correr
  atrás de um mutex do Windows entre processos, a remoção vai por certificado, e no CI a limpeza que
  falha reprova.
- F7a adiantada (decisão do Cairo): só o que não depende das lojas, do certificado da D4 nem da F6b.
  - `docs/SUPORTE.md`: para quem atende o chamado, os vinte códigos do protocolo com a frase que a
    pessoa lê, a causa no Linux e no Windows, o que fazer e quem resolve; os avisos do diagnóstico
    por sistema; e o que nunca pedir (PIN, PUK, arquivo do certificado, CPF).
  - `release.yml`: a tag `vX.Y.Z` num commit da `main` monta os pacotes Linux de produção e os zips
    das extensões, assina as somas e cria a release como RASCUNHO. O `empacotar.sh` e o
    `testar-pacotes.sh` ganharam o modo `producao`; o `somas-e-assinatura.sh` confere a assinatura
    com SÓ a chave pública do repositório. Nada disso roda de verdade antes dos IDs das lojas e da
    chave GPG das releases: até lá, o gerador de manifestos e o script das somas recusam.
  - O PROTOCOLO dizia que o `bilhete-expirado` era da extensão ou da biblioteca; hoje nenhuma ponta o
    produz (o programa responde `relogio` ao bilhete fora do prazo).
  - Divergência do plano: a F7a pedia a release pronta para publicar; ela nasce RASCUNHO, porque
    publicar é do Cairo (regra do CLAUDE.md), e o MSI do Windows fica para quando a F6b fechar.
- Auditoria da F7a adiantada (uma revisão adversarial independente e a nossa): nenhum P0, dois P1 e
  vinte P2 e P3, todos corrigidos (o Cairo escolheu os três lotes).
  - P1: o SUPORTE nunca manda apagar certificado (apagar do repositório do Windows pode levar a chave
    junto, e o que parece velho pode ser o válido), e o `chave-ausente` do Windows fica como não
    medido; a chave privada das releases mora só no ambiente `release` do GitHub, com o Cairo como
    revisor obrigatório, e não entre os segredos do repositório, que qualquer workflow de qualquer
    ramo lê.
  - SUPORTE fiel ao código: o `token-bloqueado` cobre também o PIN VENCIDO do Linux, que se troca sem
    o PUK, e o PUK também tem tentativas contadas; a `permissao-negada` não manda mais desfazer uma
    recusa que as opções não guardam; na listagem, o módulo que passa de 20 s é deixado de lado e não
    dá `tempo-esgotado`, que é da assinatura e do orçamento da extensão (o diálogo de PIN do Windows
    sem resposta); os avisos do `listar`, que a biblioteca descarta, saíram da tabela, e entraram as
    situações de certificado que o diagnóstico imprime e a linha do repositório do Windows que falhou;
    o `nativo-ausente` distingue `ausente` de `falhou` (o manifesto que não autoriza a extensão); o
    `nativo-desatualizado` é da biblioteca, que diz a peça, também no PROTOCOLO; as frases que a
    biblioteca diz sem código e os navegadores mínimos ganharam seção; o que não foi medido no
    Windows leva a marca "(não medido)"; os avisos só do Linux saíram de "Em qualquer sistema", e
    entrou o de outros sistemas.
  - A frase da biblioteca para a `permissao-negada` ("A permissão se muda nas opções da extensão")
    promete o que as opções não fazem: o conserto é na `@confidata/icp-brasil`, e o SUPORTE diz como
    é hoje.
  - Release mais dura: a tag tem de estar num commit que foi PONTA da `main` (a linha dos primeiros
    pais; o commit de um ramo mergeado é ancestral sem nunca ter sido ela), sem zero à esquerda e com
    até seis dígitos por parte (a forma da biblioteca), também no `empacotar.sh`; o programa tem de
    levar chave de bilhete de produção, com o `chaves.go` igual ao do `go generate`, e a extensão tem
    de estar na versão da tag; os testes do programa (x64 e arm64, e o arm64 passou a testar a cada
    push no `nativo.yml`) e da extensão rodam de novo sobre a tag; nenhum job usa cache, e o `npm ci`
    roda sem os scripts de instalação; release ou rascunho que já existe da tag para o workflow (a
    lista de releases, que inclui os rascunhos); a release leva exatamente os cinco arquivos de nome
    estável. O `testar-pacotes.sh producao <versão>` compara cada manifesto instalado, byte a byte,
    com o que o gerador escreve (também no modo de desenvolvimento), reprova o ID de desenvolvimento
    e o pacote `assinador-dev-*`, e confere a versão do pacote e do programa e a descrição.
  - Achado nosso: o job das extensões fazia checkout raso, e o carimbo do zip (a data do último
    commit em `extensao/` e `protocolo/`) sairia a do commit da tag, e não a que quem audita obtém do
    clone completo. O checkout é completo, e o `SOURCE_DATE_EPOCH` vai explícito e nas notas da
    release, como o README da extensão prometia.
  - A chave GPG, conferência e guarda: a impressão digital da primária fica PINADA em
    `protocolo/chave-gpg-das-releases.impressao`, e o `somas-e-assinatura.sh` recusa, antes de
    assinar, a pública com mais de uma chave primária, com parte privada ou fora do pino, e o segredo
    sem a senha, com a primária inteira (ela fica fora do CI, com o certificado de revogação), com
    mais de uma subchave de assinatura utilizável ou de outra chave; a assinatura tem de ser da chave
    pinada (o `VALIDSIG`), e a saída do gpg não é mais escondida. Quem baixa confere a impressão
    digital com a da tela de instalação e as somas com o `gpgv` num chaveiro só para isso. Provado
    com chaves descartáveis, fora do repositório, em catorze casos.
- F7a, a preparação do envio às lojas (o que não depende das lojas, do certificado da D4 nem da F6b):
  - O nome visível é **Assinador uShield** (decisão D2 do Cairo, 2026-09-27), o mesmo nas duas
    línguas: a extensão (nome e título das opções) e a descrição dos pacotes Linux. Os nomes internos
    não mudam.
  - `docs/LOJAS.md`: o que as três lojas pedem, com os textos prontos (as descrições em português e
    em espanhol, a finalidade única e a justificativa de cada permissão da Chrome Web Store, o uso de
    dados, as notas para quem revisa e as instruções de build das fontes para a loja do Firefox) e a
    lista do que falta antes de enviar.
  - A política de privacidade que as lojas publicam é uma página pública do ushield
    (`https://ushield.app/componente/privacidade`, decisão do Cairo), e o `extensao/PRIVACIDADE.md`
    passou a dizer o mesmo que ela: o PIN do Windows, que é digitado no diálogo do Windows ou do
    fabricante e que o programa não recebe; as versões, que a página recebe sem pedir; e o
    certificado inteiro, com a data de nascimento no e-CPF.
  - `docs/SUPORTE.md`: a empresa que desliga o programa por usuário (`NativeMessagingUserLevelHosts`)
    só usa o instalador por máquina, e o programa por usuário é lido antes do por máquina.
  - Divergência do plano: a política de privacidade seria uma seção da tela de instalação do
    Confidata; é uma página do ushield, porque o componente leva o nome dele.
- Auditoria da preparação do envio às lojas (uma revisão adversarial independente e a nossa): nenhum
  P0; os P1, corrigidos, eram todos texto que afirmava o que o código não faz.
  - A política (aqui e na página do ushield) dizia "nome mascarado" no diagnóstico, e o
    `assinatura.Mascarar` troca só os dígitos; dizia "só as versões" sem permissão, e o `ola` entrega
    também o sistema, a arquitetura e o estado da instalação; e subestimava o certificado, que vai
    inteiro (o NIS, o RG, o título de eleitor e o e-mail do e-CPF; o CPF e o nascimento do
    responsável do e-CNPJ), com o nome da leitora (que pode trazer o número de série) e o estado do
    PIN. Também: no Windows vão todos os certificados de assinatura do usuário, inclusive os de
    terceiros, e a chave pode estar no repositório do Windows; o PIN da leitora com teclado não passa
    pelo Assinador; e quem não abre conexão é o código da extensão e do programa (o do fabricante é de
    terceiros e roda junto).
  - A caixa de consentimento do diagnóstico no Firefox dizia "versões, sistema e leitoras, sem CPF", e
    escondia os certificados com o nome do titular e o navegador.
  - O README e o SUPORTE diziam "nome mascarado" no diagnóstico.
  - `docs/LOJAS.md`: as notas para quem revisa prometiam o programa numa release que nasce RASCUNHO e
    não diziam do estado escuro nem que o fluxo de assinatura não se exerce na revisão; o
    empacotador é o Vite, e não o esbuild; e a linha de dado pessoal da Chrome Web Store dizia menos
    do que sai.
