#!/usr/bin/env bash
# Prova os pacotes em contêiner (Docker), sem cartão: o .deb num Ubuntu 24.04 e o .rpm num Fedora
# 43. Fotografa /etc, /usr e /opt; instala; confere o programa (a versão dele é a do pacote), a
# descrição do pacote (de desenvolvimento ou não), cada manifesto instalado byte a byte contra o que o
# gerador de manifestos escreve AQUI, e o diagnóstico; remove; e reprova se a foto depois da remoção
# não for a de antes (arquivo ou pasta que sobrou, ou que sumiu).
#
#   instaladores/linux/testar-pacotes.sh <pasta com os pacotes> [producao <versão>]
#
# Sem `producao`, os pacotes são de DESENVOLVIMENTO: os manifestos esperados são os do gerador com a
# tag dev (o ID provisório da extensão), a descrição diz "DESENVOLVIMENTO", e o programa informa a
# versão do pacote sem o sufixo (`1.0.0~dev.1` informa `1.0.0`). Com `producao <versão>` (a release),
# os manifestos esperados são os do gerador de PRODUÇÃO (os IDs das lojas; sem eles, ele recusa), o ID
# de desenvolvimento não pode aparecer, a descrição não pode dizer "DESENVOLVIMENTO", o pacote e o
# programa têm de estar exatamente na versão da tag, e pacote com nome de desenvolvimento reprova.
set -euo pipefail

uso() {
  echo "uso: $0 <pasta com os pacotes> [producao <versão>]" >&2
  exit 2
}
case "$#" in
  1) MODO=dev VERSAO="" ;;
  3) [ "$2" = producao ] || uso; MODO=producao VERSAO="$3" ;;
  *) uso ;;
esac
PASTA="$(cd "$1" && pwd)"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ID_DEV="$(grep -o '"idChrome": *"[a-p]\{32\}"' "$RAIZ/protocolo/extensao-dev.json" | grep -o '[a-p]\{32\}')"

MANIFESTOS="$(mktemp -d)"
trap 'rm -rf "$MANIFESTOS"' EXIT
chmod 755 "$MANIFESTOS"
if [ "$MODO" = producao ]; then
  if ! [[ "$VERSAO" =~ ^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$ ]]; then
    echo "a versão de produção é X.Y.Z sem zero à esquerda (1.0.0), e não $VERSAO" >&2
    exit 2
  fi
  (cd "$RAIZ/nativo" && go run ./cmd/manifestos -saida "$MANIFESTOS" -programa /usr/lib/confidata-assinador/assinador)
  if grep -q "$ID_DEV" "$MANIFESTOS"/*.json; then
    echo "o manifesto de produção autoriza a extensão de desenvolvimento ($ID_DEV)" >&2
    exit 1
  fi
  for pacote in "$PASTA"/*; do
    case "$(basename "$pacote")" in
      assinador-dev-*)
        echo "pacote de desenvolvimento na pasta da release: $(basename "$pacote")" >&2
        exit 1
        ;;
    esac
  done
else
  (cd "$RAIZ/nativo" && go run -tags dev ./cmd/manifestos -saida "$MANIFESTOS" -programa /usr/lib/confidata-assinador/assinador)
fi
chmod 644 "$MANIFESTOS"/*.json

# O mesmo roteiro nos dois sistemas. Argumentos: o pacote, o tipo (deb ou rpm), o modo e a versão
# esperada (vazia no modo de desenvolvimento). Os manifestos esperados estão em /m.
read -r -d '' ROTEIRO <<'FIM' || true
set -euo pipefail
pacote="$1" tipo="$2" modo="$3" esperada="$4"
foto() { find /etc /usr /opt -xdev 2>/dev/null | sort; }
foto > /tmp/antes
if [ "$tipo" = deb ]; then
  dpkg -i "$pacote" >/dev/null
  versao_do_pacote="$(dpkg-query -W -f='${Version}' confidata-assinador)"
  descricao="$(dpkg-query -W -f='${Description}' confidata-assinador)"
else
  rpm -i "$pacote"
  versao_do_pacote="$(rpm -q --qf '%{VERSION}' confidata-assinador)"
  descricao="$(rpm -q --qf '%{SUMMARY}\n%{DESCRIPTION}' confidata-assinador)"
fi
programa=/usr/lib/confidata-assinador/assinador
versao_do_programa="$("$programa" versao)"
if [ "$modo" = producao ]; then
  if [ "$versao_do_pacote" != "$esperada" ] || [ "$versao_do_programa" != "$esperada" ]; then
    echo "a versão não é a da release: pacote $versao_do_pacote, programa $versao_do_programa, esperada $esperada" >&2
    exit 1
  fi
  if grep -qi desenvolvimento <<< "$descricao"; then
    echo "a descrição do pacote de produção diz DESENVOLVIMENTO" >&2
    exit 1
  fi
else
  if [ "$versao_do_programa" != "${versao_do_pacote%%[~-]*}" ]; then
    echo "o programa informa $versao_do_programa, e o pacote é $versao_do_pacote" >&2
    exit 1
  fi
  if ! grep -q DESENVOLVIMENTO <<< "$descricao"; then
    echo "a descrição do pacote de desenvolvimento não diz DESENVOLVIMENTO" >&2
    exit 1
  fi
fi
# Byte a byte, pelo SHA-256 (o `cmp` é do diffutils, que a imagem mínima do Fedora não tem).
igual() {
  if [ "$(sha256sum < "$1")" != "$(sha256sum < "$2")" ]; then
    echo "o manifesto instalado $1 não é o que o gerador escreve" >&2
    return 1
  fi
}
for m in /etc/opt/chrome /etc/chromium /etc/opt/edge; do
  igual "$m/native-messaging-hosts/br.com.confidata.assinador.json" /m/chromium.json
done
firefox=(/usr/lib/mozilla)
if [ "$tipo" = rpm ]; then firefox+=(/usr/lib64/mozilla); fi
for m in "${firefox[@]}"; do
  igual "$m/native-messaging-hosts/br.com.confidata.assinador.json" /m/firefox.json
done
"$programa" diagnostico >/dev/null
if [ "$tipo" = deb ]; then dpkg -r confidata-assinador >/dev/null; else rpm -e confidata-assinador; fi
foto > /tmp/depois
# comm (do coreutils, presente nas imagens mínimas): a 1a coluna é o que sumiu, a 2a o que sobrou.
diferenca="$(comm -3 /tmp/antes /tmp/depois)"
if [ -n "$diferenca" ]; then
  echo "a remoção não devolveu o sistema como estava (à esquerda sumiu, recuado sobrou):" >&2
  echo "$diferenca" >&2
  exit 1
fi
echo "ok"
FIM

achou=0
for deb in "$PASTA"/*.deb; do
  [ -e "$deb" ] || continue
  achou=1
  echo "== $(basename "$deb") no Ubuntu 24.04"
  docker run --rm -v "$PASTA:/p:ro" -v "$MANIFESTOS:/m:ro" ubuntu:24.04 bash -c "$ROTEIRO" _ "/p/$(basename "$deb")" deb "$MODO" "$VERSAO"
done
for rpm in "$PASTA"/*.rpm; do
  [ -e "$rpm" ] || continue
  achou=1
  echo "== $(basename "$rpm") no Fedora 43"
  docker run --rm -v "$PASTA:/p:ro" -v "$MANIFESTOS:/m:ro" fedora:43 bash -c "$ROTEIRO" _ "/p/$(basename "$rpm")" rpm "$MODO" "$VERSAO"
done
if [ "$achou" = 0 ]; then
  echo "nenhum pacote em $PASTA" >&2
  exit 1
fi
