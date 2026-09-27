#!/usr/bin/env bash
# As somas da release e a assinatura delas (F7a, §3.10 do plano). Escreve o `SHA256SUMS` com cada
# arquivo da pasta, assina com a subchave GPG das releases (`SHA256SUMS.asc`, destacada e em texto) e
# confere as duas coisas como quem baixa confere: `sha256sum -c`, e `gpg --verify` com SÓ a chave
# pública do repositório (`protocolo/chave-gpg-das-releases.asc`), exigindo que quem assinou seja a
# chave da impressão digital PINADA em `protocolo/chave-gpg-das-releases.impressao`.
#
#   ASSINADOR_GPG_CHAVE=<subchave, armada> ASSINADOR_GPG_SENHA=<senha> instaladores/somas-e-assinatura.sh <pasta>
#
# Antes de assinar, confere a guarda da chave (regra 13 do CLAUDE.md): a pública do repositório é UMA
# chave primária, é a da impressão pinada, e não traz parte privada; o segredo traz a primária só como
# ESBOÇO (ela fica fora do CI, com o certificado de revogação) e exatamente uma subchave de assinatura,
# cifrada com a senha. Qualquer desvio para o script antes de assinar. Sem a pública, sem a impressão,
# sem a subchave ou sem a senha, nada é assinado: release sem somas assinadas não sai. O segredo nunca
# aparece na saída; a saída do gpg aparece, para a causa de uma recusa ficar à vista.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "uso: $0 <pasta com os arquivos da release>" >&2
  exit 2
fi
PASTA="$(cd "$1" && pwd)"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PUBLICA="$RAIZ/protocolo/chave-gpg-das-releases.asc"
PINADA="$RAIZ/protocolo/chave-gpg-das-releases.impressao"

falhar() {
  echo "$1" >&2
  exit 1
}

[ -s "$PUBLICA" ] || falhar "falta a chave pública das releases em protocolo/chave-gpg-das-releases.asc (ela entra na F7a)"
[ -s "$PINADA" ] || falhar "falta a impressão digital da chave das releases em protocolo/chave-gpg-das-releases.impressao"
IMPRESSAO="$(cat "$PINADA")"
if ! [[ "$IMPRESSAO" =~ ^[0-9A-F]{40}$ || "$IMPRESSAO" =~ ^[0-9A-F]{64}$ ]]; then
  falhar "protocolo/chave-gpg-das-releases.impressao tem de ser uma linha só com a impressão digital, em hexadecimal maiúsculo, como o gpg a imprime"
fi
if grep -q 'PRIVATE KEY BLOCK' "$PUBLICA"; then
  falhar "protocolo/chave-gpg-das-releases.asc tem chave PRIVADA: tire-a do repositório e revogue a subchave"
fi
[ -n "${ASSINADOR_GPG_CHAVE:-}" ] || falhar "falta a subchave das releases em ASSINADOR_GPG_CHAVE (o segredo do ambiente release)"
[ -n "${ASSINADOR_GPG_SENHA:-}" ] || falhar "falta a senha da subchave em ASSINADOR_GPG_SENHA (o segredo do ambiente release): a subchave só vive no segredo cifrada"

# Dois chaveiros descartáveis: um só com o segredo (quem assina) e um só com a pública do
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

# A impressão digital da chave primária que vem logo depois da linha `pub` ou `sec` da listagem.
impressao_da_primaria() {
  awk -F: -v tipo="$1" '$1 == tipo { achou = 1; next } achou && $1 == "fpr" { print $10; exit }'
}

# A pública: uma chave primária só, a pinada, sem parte privada.
listagem="$(gpg --homedir "$CONFERIR" --batch --with-colons --show-keys "$PUBLICA")"
primarias="$(grep -c '^pub:' <<< "$listagem" || true)"
[ "$primarias" = 1 ] || falhar "protocolo/chave-gpg-das-releases.asc tem $primarias chaves primárias, e tem de ter uma só"
if grep -q '^sec:' <<< "$listagem"; then
  falhar "protocolo/chave-gpg-das-releases.asc tem chave PRIVADA: tire-a do repositório e revogue a subchave"
fi
da_publica="$(impressao_da_primaria pub <<< "$listagem")"
[ "$da_publica" = "$IMPRESSAO" ] || falhar "a chave pública do repositório ($da_publica) não é a da impressão pinada ($IMPRESSAO)"
gpg --homedir "$CONFERIR" --batch --quiet --import "$PUBLICA"

# O segredo: a mesma chave, com a primária só como esboço (`#` no campo 15) e uma subchave de
# assinatura (`s` nas capacidades), presente e nem vencida nem revogada.
printf '%s' "$ASSINADOR_GPG_CHAVE" | gpg --homedir "$ASSINAR" --batch --quiet --import
segredo="$(gpg --homedir "$ASSINAR" --batch --with-colons --list-secret-keys)"
[ "$(grep -c '^sec:' <<< "$segredo" || true)" = 1 ] || falhar "ASSINADOR_GPG_CHAVE tem de trazer uma chave só"
do_segredo="$(impressao_da_primaria sec <<< "$segredo")"
[ "$do_segredo" = "$IMPRESSAO" ] || falhar "a chave de ASSINADOR_GPG_CHAVE ($do_segredo) não é a da impressão pinada ($IMPRESSAO)"
if [ "$(awk -F: '$1 == "sec" { print $15 }' <<< "$segredo")" != "#" ]; then
  falhar "ASSINADOR_GPG_CHAVE traz a chave PRIMÁRIA inteira: ela fica fora do CI, e o segredo leva só a subchave de assinatura (gpg --export-secret-subkeys)"
fi
subchaves="$(awk -F: '$1 == "ssb" && $12 ~ /s/ && $15 != "#" && $2 != "e" && $2 != "r" { n++ } END { print n + 0 }' <<< "$segredo")"
[ "$subchaves" = 1 ] || falhar "ASSINADOR_GPG_CHAVE traz $subchaves subchaves de assinatura utilizáveis, e tem de trazer uma"

cd "$PASTA"
rm -f SHA256SUMS SHA256SUMS.asc
# Os nomes da release não têm espaço (§3.10: nomes estáveis); em ordem, para a lista ser a mesma em
# qualquer máquina.
mapfile -t ARQUIVOS < <(find . -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort)
if [ "${#ARQUIVOS[@]}" -eq 0 ]; then
  falhar "nenhum arquivo em $PASTA"
fi
sha256sum -- "${ARQUIVOS[@]}" > SHA256SUMS

printf '%s' "$ASSINADOR_GPG_SENHA" | gpg --homedir "$ASSINAR" --batch --quiet --yes --pinentry-mode loopback --passphrase-fd 0 --local-user "$IMPRESSAO" --armor --detach-sign --output SHA256SUMS.asc SHA256SUMS

# A conferência de quem baixa, com a pública do repositório, e a assinatura tem de ser de UMA
# subchave da chave pinada (o último campo do VALIDSIG é a primária de quem assinou).
if ! gpg --homedir "$CONFERIR" --batch --status-file "$CONFERIR/estado" --verify SHA256SUMS.asc SHA256SUMS; then
  falhar "a assinatura das somas não confere com a chave pública do repositório (a saída do gpg está acima)"
fi
validas="$(grep '^\[GNUPG:\] VALIDSIG ' "$CONFERIR/estado" || true)"
[ "$(grep -c . <<< "$validas" || true)" = 1 ] || falhar "o SHA256SUMS.asc não tem exatamente uma assinatura válida"
de_quem="$(awk '{ print $NF }' <<< "$validas")"
[ "$de_quem" = "$IMPRESSAO" ] || falhar "a assinatura é da chave $de_quem, e não da pinada ($IMPRESSAO)"
sha256sum --check --quiet SHA256SUMS
echo "SHA256SUMS e SHA256SUMS.asc conferidos contra a chave $IMPRESSAO (${#ARQUIVOS[@]} arquivo(s))"
