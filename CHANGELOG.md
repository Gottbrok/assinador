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
    diagnóstico são frases em português, como os `avisos` do `listar` (a mesma decisão da F3).
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
