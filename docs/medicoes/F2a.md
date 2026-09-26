# Medições da F2a

Cada medição tem data, equipamento e quem mediu. O que não foi medido fica marcado como pendente,
nunca presumido.

## Medido

**2026-09-25 · Claude, sem cartão na leitora** (Ubuntu 24.04 x86_64, Go 1.27.1)

- O programa de desenvolvimento, lançado como o Firefox lança um host, carrega o SafeSign 3.0 e o
  OpenSC 0.25 do catálogo, cada um no seu processo filho, e o diagnóstico os mostra `carregado`,
  com zero certificados.
- Com o SoftHSM 2.6.1 fazendo o papel do cartão (os testes de `nativo/`): a lista, a conferência
  do bilhete, o PIN errado (`pin-incorreto` com `tentativas: poucas`), o PIN certo e a assinatura
  conferida pelo `crypto/rsa` e pelo `openssl pkeyutl -verify`.
- O SoftHSM 2.6.1 marca `CKF_USER_PIN_COUNT_LOW` depois de um PIN errado, guarda a marca entre
  processos e a limpa no login certo; ele NÃO bloqueia (dez tentativas erradas seguidas, todas
  `CKR_PIN_INCORRECT`). O bloqueio e a última tentativa só se medem com cartão real.

## Pendente (o gate de saída da F2a, com o cartão e o Cairo)

Da raiz do repositório:

```sh
(cd nativo && go build -tags dev -o ../bin/assinador-dev ./cmd/assinador)
(cd ferramentas && go build -o ../bin/host-teste ./host-teste)
bin/host-teste gerar-chave
bin/host-teste listar
bin/host-teste assinar --ref <ref do certificado> --saida assinatura.bin --certificado certificado.pem
```

- [ ] O `listar` mostra o certificado do cartão Certisign, pelo SafeSign (e pelo OpenSC, se ele
      enxergar o cartão: a fusão deve deixar um só, com o provedor `pkcs11:safesign`).
- [ ] O `assinar` com o PIN produz assinatura que o `host-teste` confere e que o `openssl pkeyutl
      -verify` (o comando que ele imprime) confirma.
- [ ] Anotar aqui: `exigePin` e o `estadoDoPin` que o SafeSign declara; se a chave privada aparece
      antes do login; se o cartão tem `CKA_ALWAYS_AUTHENTICATE`; e quanto tempo o `assinar` levou.
- [ ] Um PIN errado de propósito (UMA vez, o cartão bloqueia depois de poucas): o código e as
      `tentativas` que voltam.
