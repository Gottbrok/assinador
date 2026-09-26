# Medições da F2b

Cada medição tem data, equipamento e quem mediu. O que não foi medido fica marcado como pendente,
nunca presumido.

## Medido

**2026-09-26 · Claude, sem leitora nem cartão**

- **Esta máquina** (Ubuntu 24.04.5 x86_64, pcsc-lite 2.0.3 com o `pcscd` ativo): a consulta ao
  PC/SC responde `SCARD_E_NO_READERS_AVAILABLE` (nenhuma leitora). O `assinador diagnostico` mostra o
  SafeSign e o OpenSC `carregado`, com zero certificados, e o aviso de que não há leitora. O p11-kit
  registra o OpenSC (`opensc-pkcs11.module`, `module: opensc-pkcs11.so`, resolvido em
  `/usr/lib/x86_64-linux-gnu/pkcs11/`), que é o MESMO arquivo do catálogo: a descoberta não o
  repete. Registra também o `gnome-keyring` (com `enable-in: geary, midori`) e o `p11-kit-trust`,
  que ficam de fora.
- **`pcscd` parado**, simulado com `PCSCLITE_CSOCK_NAME` apontando para um socket que não existe:
  a consulta responde `SCARD_E_NO_SERVICE` (`0x8010001D`), e o diagnóstico diz, em frase, que o
  serviço não está rodando e como iniciá-lo.
- **Os tipos do pcsc-lite** (`libpcsclite-dev` 2.0.3 do Ubuntu 24.04, cabeçalhos extraídos sem
  instalar, medidos compilando `sizeof` e `offsetof` contra eles): `DWORD` e `LONG` de 8 bytes,
  `SCARD_READERSTATE` de 80 bytes, `cbAtr` no deslocamento 32, `rgbAtr` no 40 e `MAX_ATR_SIZE` 33.
  O teste `pcsc_cabecalho` compara estas medidas e as constantes com as nossas. (A primeira versão
  deste registro dizia 64 bytes e deslocamento 28, números que não tinham sido medidos: a auditoria
  da F2b pegou, e o código sempre esteve certo.)
- **Fedora 43 x86_64 em contêiner** (OpenSC 0.27.1 e p11-kit 0.26.5 do `dnf`): o OpenSC fica em
  `/usr/lib64/opensc-pkcs11.so` (e `/usr/lib64/pkcs11/opensc-pkcs11.so` aponta para ele), o p11-kit
  o registra em `opensc.module` por nome, e ele responde a `C_GetInfo` (Cryptoki 3.0). Entrou no
  catálogo.
- **Os pacotes em contêiner:** o `.deb` (amd64) num Ubuntu 24.04 e o `.rpm` (x86_64) num Fedora 43
  instalam, o programa roda (`versao` e `diagnostico`), os manifestos ficam nas pastas dos quatro
  navegadores, e a remoção não deixa arquivo nem pasta. Sem o pcsc-lite instalado (as imagens
  mínimas não o têm), o programa abre do mesmo jeito e o diagnóstico diz que a biblioteca do PC/SC
  falta. No Fedora, a primeira versão deixava as pastas para trás (o `rpm` só apaga a pasta que o
  pacote declara); o `nfpm.yaml` passou a declará-las. Roteiro em `instaladores/linux/testar-pacotes.sh`.
- **Depois da auditoria da F2b** (2026-09-26): o roteiro fotografa `/etc`, `/usr` e `/opt` antes de
  instalar e depois de remover, e as duas fotos são iguais nos dois sistemas. Antes da correção, o
  `.deb` apagava o `/etc/opt` (pasta do sistema, sem dono no Ubuntu, que o `dpkg` tratava como do
  pacote); o `pos-remocao.sh` o devolve. O binário exige a glibc 2.34 (`objdump -T`, maior versão de
  símbolo `GLIBC_2.34`), e os pacotes a declaram.
- **Com leitora e cartão, pelo pcsc-lite FALSO** (`nativo/testes/pcsc-falso`, não é medição de
  aparelho): a consulta traz as leitoras, o cartão e o ATR; relê a lista que cresce no meio; não lê
  ATR acima de 33 bytes; marca o cartão mudo. E o host de verdade, com a biblioteca falsa escrevendo
  na saída padrão, entrega o diagnóstico inteiro pelo canal.

## Pendente (o gate de saída da F2b, com o cartão e o Cairo)

```sh
(cd nativo && go build -tags dev -o ../bin/assinador-dev ./cmd/assinador)
bin/assinador-dev diagnostico
```

- [ ] Com a leitora e o cartão Certisign: o diagnóstico mostra a leitora, o ATR, o SafeSign
      `carregado` e o certificado (o nome mascarado). Anotar aqui o ATR e o nome da leitora: o ATR
      entra em `nativo/internal/catalogo/modulos.go` (`ATRs`, com a sugestão SafeSign e esta
      medição), e o diagnóstico passa a dizer "usa o SafeSign".
- [ ] Com o `pcscd` parado (`sudo systemctl stop pcscd.socket pcscd.service`, e depois `start`): o
      aviso "O serviço pcscd não está rodando" aparece.
- [ ] (opcional) Com o SafeSign desinstalado ou renomeado e o cartão na leitora, depois de o ATR
      entrar no catálogo: o aviso "usa o SafeSign, que não está instalado".
- [ ] O `.deb` instalado de verdade no Ubuntu do Cairo (`sudo dpkg -i`), o Chrome `.deb` achando o
      host pelo manifesto de `/etc/opt/chrome/native-messaging-hosts/` (fica para a F3, com a
      extensão), e a remoção limpa (`sudo dpkg -r confidata-assinador`).
