#!/usr/bin/env bash
# Prova os pacotes em contêiner (Docker), sem cartão: o .deb num Ubuntu 24.04 e o .rpm num Fedora
# 43. Fotografa /etc, /usr e /opt; instala; confere o programa, os manifestos (o ID da extensão e o
# caminho do programa) e o diagnóstico; remove; e reprova se a foto depois da remoção não for a
# de antes (arquivo ou pasta que sobrou, ou que sumiu).
#
#   instaladores/linux/testar-pacotes.sh <pasta com os pacotes>
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "uso: $0 <pasta com os pacotes>" >&2
  exit 2
fi
PASTA="$(cd "$1" && pwd)"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ID_CHROME="$(grep -o '"idChrome": *"[a-p]\{32\}"' "$RAIZ/protocolo/extensao-dev.json" | grep -o '[a-p]\{32\}')"

# O mesmo roteiro nos dois sistemas. Argumentos: o pacote, o tipo (deb ou rpm) e o ID da extensão.
read -r -d '' ROTEIRO <<'FIM' || true
set -euo pipefail
pacote="$1" tipo="$2" id_chrome="$3"
foto() { find /etc /usr /opt -xdev 2>/dev/null | sort; }
foto > /tmp/antes
if [ "$tipo" = deb ]; then dpkg -i "$pacote" >/dev/null; else rpm -i "$pacote"; fi
programa=/usr/lib/confidata-assinador/assinador
"$programa" versao
for m in /etc/opt/chrome /etc/chromium /etc/opt/edge; do
  manifesto="$m/native-messaging-hosts/br.com.confidata.assinador.json"
  grep -q "\"chrome-extension://$id_chrome/\"" "$manifesto"
  grep -q "\"path\": \"$programa\"" "$manifesto"
done
grep -q '"assinador@confidata.com.br"' /usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json
grep -q "\"path\": \"$programa\"" /usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json
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
  docker run --rm -v "$PASTA:/p:ro" ubuntu:24.04 bash -c "$ROTEIRO" _ "/p/$(basename "$deb")" deb "$ID_CHROME"
done
for rpm in "$PASTA"/*.rpm; do
  [ -e "$rpm" ] || continue
  achou=1
  echo "== $(basename "$rpm") no Fedora 43"
  docker run --rm -v "$PASTA:/p:ro" fedora:43 bash -c "$ROTEIRO" _ "/p/$(basename "$rpm")" rpm "$ID_CHROME"
done
if [ "$achou" = 0 ]; then
  echo "nenhum pacote em $PASTA" >&2
  exit 1
fi
