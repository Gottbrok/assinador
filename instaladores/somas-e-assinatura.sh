#!/usr/bin/env bash
# As somas da release e a assinatura delas (F7a, §3.10 do plano). Escreve o `SHA256SUMS` com cada
# arquivo da pasta, assina com a chave GPG das releases (`SHA256SUMS.asc`, destacada e em texto) e
# confere as duas coisas como quem baixa confere: `sha256sum -c`, e `gpg --verify` com SÓ a chave
# pública do repositório (`protocolo/chave-gpg-das-releases.asc`). A conferência prova que a chave
# privada do segredo é a par da pública versionada, que a tela de instalação mostra.
#
#   ASSINADOR_GPG_CHAVE=<privada, armada> ASSINADOR_GPG_SENHA=<senha> instaladores/somas-e-assinatura.sh <pasta>
#
# Sem a pública no repositório ou sem a privada no ambiente, nada é assinado e o script falha:
# release sem somas assinadas não sai. A privada só existe no segredo do GitHub Actions (regra 5 do
# CLAUDE.md), e nunca aparece na saída.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "uso: $0 <pasta com os arquivos da release>" >&2
  exit 2
fi
PASTA="$(cd "$1" && pwd)"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PUBLICA="$RAIZ/protocolo/chave-gpg-das-releases.asc"

if [ ! -s "$PUBLICA" ]; then
  echo "falta a chave pública das releases em protocolo/chave-gpg-das-releases.asc (ela entra na F7a)" >&2
  exit 1
fi
if [ -z "${ASSINADOR_GPG_CHAVE:-}" ]; then
  echo "falta a chave privada das releases em ASSINADOR_GPG_CHAVE (o segredo do GitHub Actions)" >&2
  exit 1
fi

# Dois chaveiros descartáveis: um só com a privada (quem assina) e um só com a pública do
# repositório (quem confere).
ASSINAR="$(mktemp -d)"
CONFERIR="$(mktemp -d)"
limpar() {
  gpgconf --homedir "$ASSINAR" --kill gpg-agent 2>/dev/null || true
  gpgconf --homedir "$CONFERIR" --kill gpg-agent 2>/dev/null || true
  rm -rf "$ASSINAR" "$CONFERIR"
}
trap limpar EXIT
chmod 700 "$ASSINAR" "$CONFERIR"

cd "$PASTA"
rm -f SHA256SUMS SHA256SUMS.asc
# Os nomes da release não têm espaço (§3.10: nomes estáveis); em ordem, para a lista ser a mesma em
# qualquer máquina.
mapfile -t ARQUIVOS < <(find . -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort)
if [ "${#ARQUIVOS[@]}" -eq 0 ]; then
  echo "nenhum arquivo em $PASTA" >&2
  exit 1
fi
sha256sum -- "${ARQUIVOS[@]}" > SHA256SUMS

printf '%s' "$ASSINADOR_GPG_CHAVE" | gpg --homedir "$ASSINAR" --batch --quiet --import
printf '%s' "${ASSINADOR_GPG_SENHA:-}" | gpg --homedir "$ASSINAR" --batch --quiet --yes --pinentry-mode loopback --passphrase-fd 0 --armor --detach-sign --output SHA256SUMS.asc SHA256SUMS

gpg --homedir "$CONFERIR" --batch --quiet --import "$PUBLICA"
if ! gpg --homedir "$CONFERIR" --batch --verify SHA256SUMS.asc SHA256SUMS 2>/dev/null; then
  echo "a assinatura das somas não confere com a chave pública do repositório: a privada do segredo não é a par dela" >&2
  exit 1
fi
sha256sum --check --quiet SHA256SUMS
echo "SHA256SUMS e SHA256SUMS.asc conferidos (${#ARQUIVOS[@]} arquivo(s))"
