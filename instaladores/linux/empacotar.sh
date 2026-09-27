#!/usr/bin/env bash
# Monta os pacotes do Assinador para Linux: o .deb da arquitetura desta máquina e, no amd64, também o
# .rpm (x86_64). O programa é compilado AQUI, nativo (o acesso ao PKCS#11 é por cgo); os manifestos
# saem de `nativo/cmd/manifestos`, com as mesmas extensões que o programa aceita.
#
#   instaladores/linux/empacotar.sh <versão> <pasta de saída> [producao]
#
# Sem o terceiro argumento, o pacote é de DESENVOLVIMENTO (F2b): o programa com a tag dev, os IDs
# provisórios de extensão, e o nome `assinador-dev-linux-*`. Com `producao` (F7a, o `release.yml`), o
# programa sai SEM a tag dev, com os IDs das lojas e os nomes estáveis da §3.10 do plano
# (`assinador-linux-amd64.deb`, `assinador-linux-arm64.deb`, `assinador-linux-x86_64.rpm`); a versão
# tem de ser `X.Y.Z`, e sem os IDs das lojas o gerador de manifestos recusa, e o pacote não sai.
#
# Exemplos: instaladores/linux/empacotar.sh 1.0.0~dev.1 dist
#           instaladores/linux/empacotar.sh 1.0.0 dist producao
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ] || { [ "$#" -eq 3 ] && [ "$3" != producao ]; }; then
  echo "uso: $0 <versão> <pasta de saída> [producao]" >&2
  exit 2
fi
VERSAO="$1"
SAIDA="$(mkdir -p "$2" && cd "$2" && pwd)"
MODO="${3:-dev}"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NFPM="github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0"

if [ "$MODO" = producao ]; then
  # A versão publicada é exatamente `X.Y.Z`, sem zero à esquerda e com até seis dígitos por parte: é
  # a forma que a biblioteca compara com a versão mínima (fora dela, a peça nunca "atende").
  if ! [[ "$VERSAO" =~ ^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$ ]]; then
    echo "a versão de produção é X.Y.Z sem zero à esquerda (1.0.0), e não $VERSAO" >&2
    exit 2
  fi
  TAGS=""
  PREFIXO="assinador"
  # Vazio: a descrição do pacote de produção é só a do Assinador.
  export NFPM_VARIANTE=""
else
  case "$VERSAO" in
    [0-9]*) ;;
    *) echo "a versão começa por dígito (1.0.0~dev.1)" >&2; exit 2 ;;
  esac
  TAGS="dev"
  PREFIXO="assinador-dev"
  export NFPM_VARIANTE=", build de DESENVOLVIMENTO"
fi

ARQUITETURA="$(cd "$RAIZ/nativo" && go env GOARCH)"
case "$ARQUITETURA" in
  amd64|arm64) ;;
  *) echo "arquitetura $ARQUITETURA fora do pacote (amd64 ou arm64)" >&2; exit 2 ;;
esac

TRABALHO="$(mktemp -d)"
trap 'rm -rf "$TRABALHO"' EXIT

# O programa informa só o `X.Y.Z` (a forma que a biblioteca compara com a versão mínima); o sufixo
# de desenvolvimento (`~dev.N`) fica no nome do pacote, que o gerenciador de pacotes ordena ANTES da
# versão publicada.
VERSAO_DO_PROGRAMA="${VERSAO%%[~-]*}"
(cd "$RAIZ/nativo" && CGO_ENABLED=1 go build -trimpath -tags "$TAGS" -ldflags "-X main.versao=$VERSAO_DO_PROGRAMA" -o "$TRABALHO/assinador" ./cmd/assinador)
(cd "$RAIZ/nativo" && go run -tags "$TAGS" ./cmd/manifestos -saida "$TRABALHO/manifestos" -programa /usr/lib/confidata-assinador/assinador)
cp "$RAIZ/instaladores/linux/pos-remocao.sh" "$TRABALHO/pos-remocao.sh"

# O pacote declara a glibc 2.34 (nfpm.yaml). Binário que passe a exigir mais instalaria e
# quebraria ao abrir num sistema mais antigo: aqui, reprova.
GLIBC_DECLARADA=2.34
GLIBC_EXIGIDA="$(objdump -T "$TRABALHO/assinador" | grep -o 'GLIBC_[0-9.]*' | cut -d_ -f2 | sort -uV | tail -1)"
if [ -z "$GLIBC_EXIGIDA" ] || [ "$(printf '%s\n%s\n' "$GLIBC_EXIGIDA" "$GLIBC_DECLARADA" | sort -V | tail -1)" != "$GLIBC_DECLARADA" ]; then
  echo "o programa exige a glibc $GLIBC_EXIGIDA, e o pacote declara $GLIBC_DECLARADA: atualize o nfpm.yaml e este script" >&2
  exit 1
fi

export NFPM_ARQUITETURA="$ARQUITETURA" NFPM_VERSAO="$VERSAO"
# De dentro da pasta de trabalho: os `src` do nfpm.yaml são relativos a ela.
(cd "$TRABALHO" && go run "$NFPM" package --config "$RAIZ/instaladores/linux/nfpm.yaml" --packager deb --target "$SAIDA/$PREFIXO-linux-$ARQUITETURA.deb")
if [ "$ARQUITETURA" = amd64 ]; then
  (cd "$TRABALHO" && go run "$NFPM" package --config "$RAIZ/instaladores/linux/nfpm.yaml" --packager rpm --target "$SAIDA/$PREFIXO-linux-x86_64.rpm")
fi
ls -l "$SAIDA"
