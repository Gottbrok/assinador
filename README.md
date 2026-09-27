# Assinador

Componente para assinar com certificado digital A3 (token ou cartão ICP-Brasil) a partir do
navegador, sem que a chave saia do dispositivo da pessoa.

É formado por um programa nativo (Windows, Linux e, depois, macOS) e uma extensão para Chrome, Edge e
Firefox. A página nunca fala com o cartão: ela pede à extensão, que pede ao programa, que só assina um
resumo acompanhado de um bilhete emitido pelo servidor que preparou o documento. A janela de
confirmação da extensão mostra à pessoa o documento verdadeiro antes de pedir o PIN.

O programa não abre porta de rede, não acessa a internet e não guarda o PIN.

Em construção. Licença Apache-2.0.

## Estrutura

| Pasta | O que é |
|---|---|
| `nativo/` | O programa, em Go. `cmd/assinador` (modos host, módulo e diagnóstico), `cmd/manifestos` (os manifestos dos navegadores) e os pacotes de `internal/` |
| `extensao/` | A extensão para Chrome, Edge e Firefox: a ponte da página, a permissão por endereço, a janela de confirmação e as opções. Ver [`extensao/README.md`](extensao/README.md) e [`extensao/PRIVACIDADE.md`](extensao/PRIVACIDADE.md) |
| `protocolo/` | [`PROTOCOLO.md`](protocolo/PROTOCOLO.md), as chaves públicas de produção, a chave pública da extensão de desenvolvimento e as fixtures do bilhete |
| `instaladores/linux/` | O `.deb` e o `.rpm` (`empacotar.sh`) e a prova deles em contêiner (`testar-pacotes.sh`) |
| `instaladores/windows/` | Os MSI por usuário e por máquina (`Assinador.wxs`, montado pelo `empacotar.ps1`) e a prova deles no Windows (`testar-instalador.ps1`) |
| `ferramentas/` | Prova e medição (F0), o `host-teste`, que fala com o programa como a extensão e serve a página de teste da extensão (`servir`), e o `registrar-windows.ps1`, que registra o programa de desenvolvimento no Windows antes do MSI. Nunca vai para release |
| `docs/medicoes/` | O que foi medido com cartão real, com data e equipamento |

A segurança do programa está em [`SECURITY.md`](SECURITY.md).

## Compilar e testar (Linux)

Go na versão de `nativo/go.mod`, `gcc` (o acesso ao PKCS#11 é por cgo), e, para os testes que fazem
o papel do cartão, o SoftHSM2 (`softhsm2` no Debian e no Ubuntu).

```sh
cd nativo
go vet ./... && go vet -tags dev ./...
go test ./...                 # os testes com SoftHSM2 pulam sem ele; ASSINADOR_SOFTHSM=<caminho do .so> aponta um
go test -tags dev ./...
go build -o ../bin/assinador ./cmd/assinador            # release: só chaves de produção
go build -tags dev -o ../bin/assinador-dev ./cmd/assinador  # desenvolvimento: aceita chave dev e localhost
```

`go generate ./internal/bilhete` regenera `internal/bilhete/chaves.go` a partir de
`protocolo/chaves-publicas.json`; um teste reprova se os dois divergirem. O desenho das estruturas
do PC/SC se confere contra os cabeçalhos do pcsc-lite (`libpcsclite-dev`):
`CGO_CFLAGS="$(pkg-config --cflags libpcsclite)" go test -tags pcsc_cabecalho ./internal/pcsc/`.

## Diagnóstico

```sh
bin/assinador diagnostico          # em frases, para o suporte (sem CPF)
bin/assinador diagnostico --json   # o mesmo relatório, em JSON
```

Mostra o sistema, as leitoras e o ATR de cada cartão (com o programa do fabricante que o lê,
quando o ATR está no catálogo medido), os programas de cartão (módulos PKCS#11) com o estado de
cada um, os certificados com o nome mascarado, e avisos em frase ("o serviço pcscd não está
rodando", "este cartão usa o SafeSign, que não está instalado").

## Pacotes para Linux

```sh
instaladores/linux/empacotar.sh 1.0.0~dev.1 dist   # o .deb desta arquitetura e, no amd64, o .rpm
instaladores/linux/testar-pacotes.sh dist          # instala, roda e remove em contêiner (Docker)
```

São pacotes de DESENVOLVIMENTO: o programa com a tag `dev` e os manifestos com o ID provisório da
extensão. Os de produção, assinados e com os IDs das lojas, vêm com a publicação.

## Windows (desenvolvimento)

No Windows o programa lê o repositório de certificados do usuário e assina pelo CNG ou pelo CSP do
fabricante, sem PKCS#11 e sem cgo: ele compila cruzado de qualquer sistema. O PIN é pedido pelo
diálogo do próprio Windows.

```sh
cd nativo
GOOS=windows GOARCH=amd64 go build -tags dev -o ../bin/assinador-dev.exe ./cmd/assinador
GOOS=windows GOARCH=amd64 go build -tags dev -o ../bin/manifestos-dev.exe ./cmd/manifestos
```

O CI publica os dois, com o `registrar-windows.ps1`, no artefato `assinador-dev-windows-amd64` (e
`-arm64`). No Windows, sem administrador. O que veio da internet é marcado e o PowerShell, por padrão,
não roda script: primeiro se desbloqueiam os arquivos, e o script roda com a política liberada só
para aquela execução:

```powershell
Unblock-File .\assinador-dev.exe, .\manifestos-dev.exe, .\registrar-windows.ps1
powershell -ExecutionPolicy Bypass -File .\registrar-windows.ps1 -Programa .\assinador-dev.exe -Manifestos .\manifestos-dev.exe
& "$env:LOCALAPPDATA\ConfidataAssinadorDev\assinador.exe" diagnostico
powershell -ExecutionPolicy Bypass -File .\registrar-windows.ps1 -Remover
```

O registro copia o programa para `%LOCALAPPDATA%\ConfidataAssinadorDev` (e tira da cópia a marca de
"veio da internet"), gera os manifestos (os IDs de extensão que o programa aceita) e grava as chaves
`HKCU` do Chrome, do Edge, do Chromium e do Firefox; `-Remover` desfaz tudo. O programa de
desenvolvimento não é assinado (a assinatura de código espera o certificado), e o SmartScreen pode pedir
confirmação na primeira execução. A chave dev do bilhete fica em
`%APPDATA%\confidata-assinador\chaves-dev.json`.

### O MSI de desenvolvimento

O CI publica também, no mesmo artefato, dois MSI de DESENVOLVIMENTO por arquitetura (o programa com
a tag `dev` e os manifestos com o ID provisório da extensão), nenhum assinado até o certificado de
assinatura de código:

| MSI | Instala em | Chaves | Para quem |
|---|---|---|---|
| `assinador-dev-windows-<arq>-usuario.msi` | `%LOCALAPPDATA%\Programs\Confidata Assinador` | `HKCU` | a pessoa, sem administrador (dois cliques ou `msiexec /i`) |
| `assinador-dev-windows-<arq>-maquina.msi` | `%ProgramFiles%\Confidata Assinador` | `HKLM` | o administrador, por GPO ou Intune |

Os dois levam o programa e os dois manifestos na mesma pasta, e as chaves do Chrome, do Edge, do
Chromium e do Firefox apontam para os manifestos; os manifestos apontam o programa pelo NOME, que os
navegadores resolvem a partir da pasta do manifesto.

O MSI por usuário se instala com dois cliques; sem assinatura, o SmartScreen pode pedir confirmação
("Mais informações", "Executar assim mesmo"), e o `Unblock-File` no arquivo baixado a evita. O por
máquina se instala em silêncio num prompt de ADMINISTRADOR (fora dele, o `/qn` falha sem perguntar
nada):

```powershell
msiexec /i assinador-dev-windows-amd64-maquina.msi /qn
```

A versão nova se instala por cima da instalada (o mesmo escopo; a mesma versão também, o que troca o
x64 emulado pelo arm64). Para remover, "Configurações, Aplicativos, Assinador Confidata
(desenvolvimento)", ou `msiexec /x <arquivo> /qn` com o MESMO arquivo que instalou: cada MSI montado
é um produto novo, e o de outra execução do CI responde 1605.

Três cuidados:
- **Um mecanismo de cada vez.** O MSI por usuário e o `registrar-windows.ps1` gravam as mesmas chaves
  `HKCU`: rode `registrar-windows.ps1 -Remover` antes do MSI. E o Chrome e o Edge leem `HKCU` antes de
  `HKLM`, então, com o por usuário e o por máquina instalados, o navegador abre o por usuário (um MSI
  por máquina não remove o por usuário). Onde a política `NativeMessagingUserLevelHosts` do Chrome
  estiver desligada (organização gerenciada), só o MSI por máquina funciona.
- **Duas versões.** O PACOTE de desenvolvimento tem a versão `0.1.<execução do CI>`, abaixo de
  qualquer publicação; o PROGRAMA informa a da próxima publicação (hoje `1.0.0`), que é a que a
  biblioteca compara com a versão mínima, como os pacotes do Linux (`1.0.0~dev.N`). Um MSI de
  execução mais antiga não se instala por cima de um mais novo ("Uma versão mais nova do Assinador
  já está instalada"): remova antes.
- **A chave dev do bilhete** continua em `%APPDATA%\confidata-assinador\chaves-dev.json`, de cada
  pessoa que for assinar, também no MSI por máquina.

Montar, validar e provar os MSI é no Windows (o WiX monta MSI só lá), no PowerShell 7 (`pwsh`), com o
Go do `nativo/go.mod` e o WiX 5.0.2 sobre o .NET 8 (`dotnet tool install --global wix --version
5.0.2`; o 6 exige o EULA da taxa de manutenção da OSMF):

```powershell
.\instaladores\windows\empacotar.ps1 -VersaoDoMsi 0.1.1 -VersaoDoPrograma 1.0.0 -Saida dist -Arquitetura amd64
.\instaladores\windows\testar-instalador.ps1 -Msi dist\assinador-dev-windows-amd64-usuario.msi -Escopo usuario -VersaoDoPrograma 1.0.0
```

O `empacotar.ps1` roda os ICE (`wix msi validate`, que o `wix build` do WiX 5 não roda). O
`testar-instalador.ps1` confere a estrutura do pacote (no por usuário: nada exige elevação, todo
valor em `HKCU`, nenhuma pasta de máquina), parte de uma máquina sem o Assinador, instala (com
`-MsiAnterior` e `-VersaoAnteriorDoPrograma`, primeiro a versão anterior, para provar a
atualização), confere o produto, as chaves, os manifestos e o programa, conversa com ele como o
Chrome conversa, desinstala e reprova se sobrar arquivo, pasta ou chave. O CI roda os dois escopos
nas duas arquiteturas; o executor é administrador, então instalar numa conta comum e abrir pelo
navegador de verdade são do teste manual. O MSI de produção, com os IDs das lojas e assinado, é da
publicação, e atualiza o de desenvolvimento do mesmo escopo.

Os testes do Windows rodam no CI (job `windows`, em x64 e arm64): certificados com chave de
SOFTWARE criados pelo `New-SelfSignedCertificate` fazem o papel do cartão, um no caminho CNG e
outro no CSP. Fora do Windows, `GOOS=windows go vet ./...` e o `staticcheck` com `GOOS=windows`
(compilado para a máquina, e não por `go run`, que o compilaria para o Windows) pegam o que não
compila.

## Testar com o cartão, sem navegador

```sh
(cd nativo && go build -tags dev -o ../bin/assinador-dev ./cmd/assinador)
(cd ferramentas && go build -o ../bin/host-teste ./host-teste)
bin/host-teste gerar-chave        # par ES256 de desenvolvimento, fora do repositório
bin/host-teste listar             # a ref de cada certificado
bin/host-teste assinar --ref <ref> --saida assinatura.bin --certificado certificado.pem
```

O `assinar` mostra o que a janela de confirmação mostraria, pede o PIN no terminal (quando o
cartão o exige), confere a assinatura com o certificado e imprime o comando do `openssl` para
conferir de novo. O middleware do cartão que não estiver no catálogo medido entra, um caminho por
linha, em `~/.config/confidata-assinador/modulos`.
