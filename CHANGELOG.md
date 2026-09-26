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
