#!/usr/bin/env bash
# Instala o host de PROVA (br.com.confidata.assinador.prova) para a medição (c) da F0.
# Compila o host e grava o manifesto nos lugares que cada navegador lê. Desfaz com desinstalar.sh.
set -euo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOME="br.com.confidata.assinador.prova"
ID_CHROME="dgmccefbillcnlpgplakclbjpjpdjdlh"   # derivado da "key" do manifesto da extensão
ID_FIREFOX="prova-assinador@confidata.com.br"
BIN="$AQUI/bin/host"

GO="${GO:-$(command -v go || echo "$HOME/.local/go/bin/go")}"
mkdir -p "$AQUI/bin"
(cd "$AQUI/.." && "$GO" build -o "$BIN" ./prova-mensagens/host)
chmod 0755 "$BIN"

manifesto_chrome() {
  cat <<JSON
{
  "name": "$NOME",
  "description": "Host de prova do Assinador (F0)",
  "path": "$1",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://$ID_CHROME/"]
}
JSON
}

manifesto_firefox() {
  cat <<JSON
{
  "name": "$NOME",
  "description": "Host de prova do Assinador (F0)",
  "path": "$1",
  "type": "stdio",
  "allowed_extensions": ["$ID_FIREFOX"]
}
JSON
}

gravar() {
  mkdir -p "$1"
  printf '%s\n' "$2" > "$1/$NOME.json"
  echo "  $1/$NOME.json"
}

echo "host: $BIN"
echo "manifestos:"
gravar "$HOME/.config/google-chrome/NativeMessagingHosts" "$(manifesto_chrome "$BIN")"
gravar "$HOME/.config/chromium/NativeMessagingHosts" "$(manifesto_chrome "$BIN")"
# Chromium em Snap só lê dentro do diretório dele; o binário também vai para lá, porque o
# confinamento pode não deixar executar fora.
SNAP_CHROMIUM="$HOME/snap/chromium/common"
if [ -d "$HOME/snap/chromium" ] || { command -v snap >/dev/null && snap list chromium >/dev/null 2>&1; }; then
  mkdir -p "$SNAP_CHROMIUM/assinador-prova"
  cp "$BIN" "$SNAP_CHROMIUM/assinador-prova/host"
  gravar "$SNAP_CHROMIUM/chromium/NativeMessagingHosts" "$(manifesto_chrome "$SNAP_CHROMIUM/assinador-prova/host")"
fi
# Firefox (inclusive o Snap, que lança o host fora do confinamento pelo portal do xdg-desktop-portal).
gravar "$HOME/.mozilla/native-messaging-hosts" "$(manifesto_firefox "$BIN")"
echo "pronto. Carregue a extensão de $AQUI/extensao em cada navegador."
