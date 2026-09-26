#!/usr/bin/env bash
# Monta os pacotes de DESENVOLVIMENTO do Assinador para Linux (F2b): o .deb da arquitetura desta
# máquina e, no amd64, também o .rpm (x86_64). O programa é compilado AQUI, nativo (o acesso ao
# PKCS#11 é por cgo), com a tag dev; os manifestos saem de `nativo/cmd/manifestos`, com as mesmas
# extensões que o programa aceita.
#
#   instaladores/linux/empacotar.sh <versão> <pasta de saída>
#
# Exemplo: instaladores/linux/empacotar.sh 0.1.0~dev.1 dist
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "uso: $0 <versão> <pasta de saída>" >&2
  exit 2
fi
VERSAO="$1"
SAIDA="$(mkdir -p "$2" && cd "$2" && pwd)"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NFPM="github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0"

case "$VERSAO" in
  [0-9]*) ;;
  *) echo "a versão começa por dígito (0.1.0~dev.1)" >&2; exit 2 ;;
esac

ARQUITETURA="$(cd "$RAIZ/nativo" && go env GOARCH)"
case "$ARQUITETURA" in
  amd64|arm64) ;;
  *) echo "arquitetura $ARQUITETURA fora do pacote (amd64 ou arm64)" >&2; exit 2 ;;
esac

TRABALHO="$(mktemp -d)"
trap 'rm -rf "$TRABALHO"' EXIT

(cd "$RAIZ/nativo" && CGO_ENABLED=1 go build -trimpath -tags dev -ldflags "-X main.versao=$VERSAO" -o "$TRABALHO/assinador" ./cmd/assinador)
(cd "$RAIZ/nativo" && go run -tags dev ./cmd/manifestos -saida "$TRABALHO/manifestos" -programa /usr/lib/confidata-assinador/assinador)

export NFPM_ARQUITETURA="$ARQUITETURA" NFPM_VERSAO="$VERSAO"
# De dentro da pasta de trabalho: os `src` do nfpm.yaml são relativos a ela.
(cd "$TRABALHO" && go run "$NFPM" package --config "$RAIZ/instaladores/linux/nfpm.yaml" --packager deb --target "$SAIDA/assinador-dev-linux-$ARQUITETURA.deb")
if [ "$ARQUITETURA" = amd64 ]; then
  (cd "$TRABALHO" && go run "$NFPM" package --config "$RAIZ/instaladores/linux/nfpm.yaml" --packager rpm --target "$SAIDA/assinador-dev-linux-x86_64.rpm")
fi
ls -l "$SAIDA"
