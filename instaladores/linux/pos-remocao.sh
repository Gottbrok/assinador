#!/bin/sh
# Depois da remoção do .deb. O dpkg apaga, ao remover, toda pasta vazia que o pacote trouxe e que
# nenhum outro pacote declara; o /etc/opt (pasta do sistema, da FHS) existe antes do pacote sem
# dono nenhum, e não pode sumir por causa dele. O rpm não precisa disto: ele só apaga as pastas que
# o pacote declara, e o /etc/opt não está entre elas.
set -e
case "$1" in
  remove|purge)
    [ -d /etc/opt ] || install -d -m 0755 /etc/opt
    ;;
esac
exit 0
