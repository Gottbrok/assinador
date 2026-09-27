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
| `ferramentas/` | Prova e medição (F0), o `host-teste`, que fala com o programa como a extensão e serve a página de teste da extensão (`servir`), e o `registrar-windows.ps1`, que registra o programa de desenvolvimento no Windows antes do MSI. Nunca vai para release |
| `docs/medicoes/` | O que foi medido com cartão real, com data e equipamento |

A segurança do programa está em [`SECURITY.md`](SECURITY.md). Para quem atende o chamado de quem não
consegue assinar, cada código de erro e cada aviso do diagnóstico, com a causa e o que fazer, está em
[`docs/SUPORTE.md`](docs/SUPORTE.md).

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
extensão. Os de produção (`empacotar.sh <X.Y.Z> dist producao`) só saem pela release, e só depois de
os IDs das lojas existirem: antes disso, o gerador de manifestos recusa.

## Release

Uma tag `vX.Y.Z` num commit da `main` roda o `.github/workflows/release.yml`: os pacotes Linux de
produção (x64 e arm64, provados em contêiner com os IDs das lojas), os zips das extensões para as lojas
(reproduzíveis), o `SHA256SUMS` e o `SHA256SUMS.asc`. A release nasce como RASCUNHO, e quem a publica é
o Cairo, depois de conferir. Os nomes são estáveis, para o endereço
`https://github.com/Gottbrok/assinador/releases/latest/download/<arquivo>` não mudar entre versões.

A assinatura das somas é conferida, antes de a release existir, com SÓ a chave pública de
`protocolo/chave-gpg-das-releases.asc`: a privada vive no segredo `ASSINADOR_GPG_CHAVE` (com a senha em
`ASSINADOR_GPG_SENHA`), e sem as duas o workflow para. Quem baixa confere assim:

```sh
gpg --import chave-gpg-das-releases.asc
gpg --verify SHA256SUMS.asc SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS
```

O MSI do Windows entra na release quando o instalador (F6b) existir.

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
desenvolvimento não é assinado (a assinatura de código é da F6b), e o SmartScreen pode pedir
confirmação na primeira execução. A chave dev do bilhete fica em
`%APPDATA%\confidata-assinador\chaves-dev.json`.

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
