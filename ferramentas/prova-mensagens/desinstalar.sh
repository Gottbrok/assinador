#!/usr/bin/env bash
# Remove o host de PROVA instalado por instalar.sh. Não toca em nenhum outro manifesto.
set -euo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOME="br.com.confidata.assinador.prova"

for d in \
  "$HOME/.config/google-chrome/NativeMessagingHosts" \
  "$HOME/.config/chromium/NativeMessagingHosts" \
  "$HOME/snap/chromium/common/chromium/NativeMessagingHosts" \
  "$HOME/.mozilla/native-messaging-hosts"; do
  if [ -f "$d/$NOME.json" ]; then
    rm -f "$d/$NOME.json"
    echo "removido $d/$NOME.json"
  fi
done
rm -rf "$HOME/snap/chromium/common/assinador-prova" "$AQUI/bin"
echo "pronto."
