#!/usr/bin/env bash
# Prova os pacotes em contêiner (Docker), sem cartão: o .deb num Ubuntu 24.04 e o .rpm num Fedora
# 43. Instala; confere o programa, os manifestos e o diagnóstico; remove; e reprova se sobrar
# arquivo ou pasta do pacote.
#
#   instaladores/linux/testar-pacotes.sh <pasta com os pacotes>
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "uso: $0 <pasta com os pacotes>" >&2
  exit 2
fi
PASTA="$(cd "$1" && pwd)"

# O mesmo roteiro nos dois sistemas; muda só como se instala e se remove.
read -r -d '' ROTEIRO <<'FIM' || true
set -euo pipefail
instalar
programa=/usr/lib/confidata-assinador/assinador
"$programa" versao
for m in /etc/opt/chrome /etc/chromium /etc/opt/edge; do
  grep -q '"allowed_origins"' "$m/native-messaging-hosts/br.com.confidata.assinador.json"
done
grep -q '"assinador@confidata.com.br"' /usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json
grep -q "\"path\": \"$programa\"" /usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json
"$programa" diagnostico >/dev/null
remover
sobras=""
for d in /usr/lib/confidata-assinador /etc/opt/chrome /etc/chromium /etc/opt/edge /usr/lib/mozilla /usr/lib64/mozilla; do
  [ -e "$d" ] && sobras="$sobras $d"
done
if [ -n "$sobras" ]; then
  echo "sobrou depois da remoção:$sobras" >&2
  exit 1
fi
echo "ok"
FIM

achou=0
for deb in "$PASTA"/*.deb; do
  [ -e "$deb" ] || continue
  achou=1
  echo "== $(basename "$deb") no Ubuntu 24.04"
  docker run --rm -v "$PASTA:/p:ro" ubuntu:24.04 bash -c "instalar() { dpkg -i /p/$(basename "$deb") >/dev/null; }; remover() { dpkg -r confidata-assinador >/dev/null; }; $ROTEIRO"
done
for rpm in "$PASTA"/*.rpm; do
  [ -e "$rpm" ] || continue
  achou=1
  echo "== $(basename "$rpm") no Fedora 43"
  docker run --rm -v "$PASTA:/p:ro" fedora:43 bash -c "instalar() { rpm -i /p/$(basename "$rpm"); }; remover() { rpm -e confidata-assinador; }; $ROTEIRO"
done
if [ "$achou" = 0 ]; then
  echo "nenhum pacote em $PASTA" >&2
  exit 1
fi
