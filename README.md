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
| `nativo/` | O programa, em Go. `cmd/assinador` (modos host e módulo) e os pacotes de `internal/` |
| `protocolo/` | [`PROTOCOLO.md`](protocolo/PROTOCOLO.md), as chaves públicas de produção e as fixtures do bilhete |
| `ferramentas/` | Prova e medição (F0) e o `host-teste`, que fala com o programa como a extensão. Nunca vai para release |
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
`protocolo/chaves-publicas.json`; um teste reprova se os dois divergirem.

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
