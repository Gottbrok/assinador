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
- F6b-i: os MSI de DESENVOLVIMENTO do Windows, sem assinatura (a assinatura de código é a F6b-ii,
  quando o certificado chegar).
  - `instaladores/windows/Assinador.wxs` (WiX 5.0.2): um fonte e dois MSI, o por usuário
    (`%LOCALAPPDATA%\Programs\Confidata Assinador` e `HKCU`, sem pedir administrador) e o por máquina
    (`Program Files` e `HKLM`, para GPO e Intune), em x64 e arm64, com `MajorUpgrade`. Um
    componente só: o programa, os dois manifestos e as chaves do Chrome, do Edge, do Chromium e do
    Firefox instalam e saem juntos. Os `UpgradeCode` (um por escopo, os mesmos nas duas arquiteturas
    e no MSI de produção) e os GUIDs dos componentes são nomes fixos.
  - `cmd/manifestos -relativo`: o manifesto aponta o programa só pelo NOME, na mesma pasta, que os
    navegadores resolvem no Windows; recusa pasta, `..`, separador e letra de unidade.
  - `empacotar.ps1` monta os dois MSI da arquitetura pedida (programa `dev`, versão `X.Y.Z` do
    Windows Installer igual à que o programa informa); `testar-instalador.ps1` prova cada um: o MSI
    por usuário não exige elevação (resumo do pacote e `ALLUSERS`), a versão anterior instalada e
    atualizada, as chaves, os manifestos e o programa conferidos, o olá pelo caminho que o navegador
    segue, e a desinstalação sem sobra de arquivo, pasta ou chave (a foto do registro antes e depois).
  - CI: o job `windows` monta e prova os dois escopos nas duas arquiteturas, sobre o .NET 8 do
    `setup-dotnet`, e publica os MSI no artefato `assinador-dev-windows-<arq>`.
  - Divergências do plano, decididas na implementação (com o Cairo): a F6b anda em duas partes,
    porque o certificado da assinatura de código ainda não foi contratado; o MSI desta parte é o de
    desenvolvimento (o de produção é da publicação); o WiX é o 5.0.2, e não o 6, que exige o EULA da
    taxa de manutenção da OSMF; e a medição das chaves de extensão externa do Chrome e do Edge
    precisa do ID da loja e vai para a publicação.
- Auditoria da F6b-i (uma revisão adversarial independente e a nossa), antes do primeiro CI da fase:
  nenhum P0. Dois P1 corrigidos:
  - o `testar-instalador.ps1` quebraria no primeiro CI: o PowerShell desenrola objeto COM enumerável
    que sai de função, e o StringList do `RelatedProducts` virava `$null` (zero produtos) ou o texto
    do ProductCode (um). O resultado do `InvokeMember` sai inteiro, pela vírgula, e as listas se leem
    para variável;
  - o programa do MSI informava a versão do PACOTE (`0.1.N`), e a biblioteca, que exige a versão
    mínima `1.0.0`, diria à pessoa que o Assinador está desatualizado. O MSI passou a ter duas
    versões, como o Linux (`-VersaoDoMsi` e `-VersaoDoPrograma`), e a atualização se prova pela
    versão do produto instalado e pela troca do programa (a 0.0.1 informa `0.0.1`).
  Os P2 e P3, corrigidos por decisão do Cairo:
  - os ICE não rodavam (no WiX 5 o `wix build` não valida): o `empacotar.ps1` roda o
    `wix msi validate` em cada MSI;
  - a atualização aceita a mesma versão e ignora o idioma (`AllowSameVersionUpgrades`,
    `IgnoreLanguage`): o arm64 troca o x64 emulado da mesma versão, a mesma execução do CI rodada de
    novo não duplica, e um MSI de produção em outro idioma acha este;
  - a estrutura do MSI por usuário é conferida além do resumo (todo valor em `HKCU`, nenhuma pasta de
    máquina), e a do por máquina ao contrário; instalar numa conta comum segue com o teste manual;
  - a conversa leva os argumentos que o Chrome passa no Windows (`--parent-window=0`), com origem
    neutra, e mostra o erro padrão do programa quando falha; o texto não promete mais "o mesmo
    caminho do navegador";
  - o README: a remoção por "Configurações, Aplicativos" (o `msiexec /x` só funciona com o mesmo
    arquivo), o `/qn` do por máquina num prompt de administrador, o `HKCU` que o Chrome e o Edge leem
    antes do `HKLM` (o por usuário esconde o por máquina), a política `NativeMessagingUserLevelHosts`
    e as duas versões;
  - o `-relativo` do gerador de manifestos aceita só o nome de um `.exe` em lista branca, e recusa
    nome de dispositivo do Windows;
  - o `empacotar.ps1` devolve as variáveis do Go da sessão e recusa zero à esquerda na versão;
  - o CI confere que o WiX é o 5.0.2, o msiexec tenta de novo no 1618, a falha mostra o trecho do log
    em volta do "Return value 3", e os MSI e os logs sobem como artefato quando algo falha;
  - a pasta-mãe e as chaves-mãe que já existiam VAZIAS podem sumir na desinstalação (a regra do
    Windows Installer apaga a chave que fica vazia, sem olhar quem a criou), e isso não reprova mais;
  - o resumo do pacote sai em português.
  - Aceito: o título do commit da F6b-i diz "provados no CI" antes de o CI rodar (o
    `docs/medicoes/F6b.md` diz o que foi e o que não foi provado).
- F6b-i: o nome visível Assinador uShield (decisão D2) também no MSI: o produto em "Aplicativos"
  ("Assinador uShield (desenvolvimento)"), o resumo do pacote, a mensagem de versão mais nova e a pasta
  de instalação nos dois escopos (`%LOCALAPPDATA%\Programs\Assinador uShield` e
  `%ProgramFiles%\Assinador uShield`, antes `Confidata Assinador`). Os nomes internos não mudam, e o
  `Manufacturer` segue Confidata, como o `vendor` dos pacotes Linux. A mensagem do `.wxs` sem versão
  passou a dizer que a versão é a do pacote (a auditoria separou as duas).
