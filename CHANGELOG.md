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
