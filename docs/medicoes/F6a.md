# Medições da F6a

Cada medição tem data, equipamento e quem mediu. O que não foi medido fica marcado como pendente,
nunca presumido.

## Medido

**2026-09-26 · Claude, sem Windows, sem leitora nem cartão**

- **Os cabeçalhos do SDK do Windows** (os do MinGW-w64 11.0.1 do Ubuntu 24.04, com o GCC 13.2
  para `x86_64-w64-mingw32`, extraídos sem instalar e medidos compilando para assembly e lendo o
  valor que o compilador pôs em cada constante):
  - `SCARD_READERSTATEW` tem 64 bytes: `szReader` no 0, `pvUserData` no 8, `dwCurrentState` no 16,
    `dwEventState` no 20, `cbAtr` no 24 e `rgbAtr` no 28, com 36 bytes (no pcsc-lite do Linux são
    33); `SCARD_ATR_LENGTH` é 33 e `SCARD_SCOPE_USER` é 0.
  - `CRYPT_KEY_PROV_INFO` tem 48 bytes: `pwszContainerName` no 0, `pwszProvName` no 8,
    `dwProvType` no 16 e `dwKeySpec` no 40. `BCRYPT_PKCS1_PADDING_INFO` tem 8.
  - Os códigos que o provedor distingue são os escritos no código: `SCARD_E_CANCELLED`
    `0x80100002`, `SCARD_E_NO_SMARTCARD` `0x8010000C`, `SCARD_W_REMOVED_CARD` `0x80100069`,
    `SCARD_W_WRONG_CHV` `0x8010006B`, `SCARD_W_CHV_BLOCKED` `0x8010006C`,
    `SCARD_W_CANCELLED_BY_USER` `0x8010006E`, `NTE_BAD_ALGID` `0x80090008`, `NTE_NO_KEY`
    `0x8009000D`, `NTE_PERM` `0x80090010`, `NTE_BAD_KEYSET` `0x80090016`, `NTE_KEYSET_NOT_DEF`
    `0x80090019`, `NTE_NOT_SUPPORTED` `0x80090029`, `NTE_USER_CANCELLED` `0x80090036`,
    `CRYPT_E_NO_KEY_PROPERTY` `0x8009200B`, e os `HRESULT_FROM_WIN32` de `ERROR_ACCESS_DENIED`
    (`0x80070005`) e `ERROR_CANCELLED` (`0x800704C7`); os do PC/SC (`SCARD_E_NO_SERVICE`,
    `SERVICE_STOPPED`, `NO_READERS_AVAILABLE`, `TIMEOUT`, `INSUFFICIENT_BUFFER`) são os mesmos do
    pcsc-lite, e `SCARD_STATE_PRESENT` e `MUTE` também.
  - `CERT_KEY_PROV_INFO_PROP_ID` 2, `PP_CLIENT_HWND` 1, `CALG_SHA_256` `0x800C`, `HP_HASHVAL` 2,
    `BCRYPT_PAD_PKCS1` 2, `CERT_NCRYPT_KEY_SPEC` `0xFFFFFFFF`.

  O CI do Windows confere de novo, rodando, contra o MinGW do executor (tags `pcsc_cabecalho` e
  `windows_cabecalho`).
- **Compilação:** o programa, os testes e os dois testes de cabeçalho (estes com o cgo e o MinGW)
  compilam para `windows/amd64` e `windows/arm64`, nos builds de release e `dev`, com `go vet`,
  `staticcheck` e `govulncheck` limpos. RODAR no Windows é do CI (job `windows`) e da máquina do Cairo.

## Pendente (a prova da F0 no Windows e o gate de saída da F6a, com o cartão e o Cairo)

O programa, o gerador de manifestos e o `registrar-windows.ps1` saem do CI, no artefato
`assinador-dev-windows-amd64`. Registro e diagnóstico, no PowerShell, sem administrador:

```powershell
.\registrar-windows.ps1 -Programa .\assinador-dev.exe -Manifestos .\manifestos-dev.exe
& "$env:LOCALAPPDATA\ConfidataAssinadorDev\assinador.exe" diagnostico
```

- [ ] **(d) da F0:** com o cartão Certisign e o SafeSign para Windows, o certificado aparece em
      `CurrentUser\My` (o `certmgr.msc`, e a lista do diagnóstico), com o provedor que o SafeSign
      registra: anotar aqui o NOME do provedor (é ele que entra no catálogo do Windows) e se é CSP
      legado ou KSP (`windows:csp` ou `windows:cng` na lista), e se o SHA-256 funciona.
- [ ] **(e) da F0:** o diálogo de PIN do provedor abre na FRENTE do Chrome e do Edge (a janela-mãe
      que eles passam), e anotar onde ele abre no Firefox, que não passa janela.
- [ ] O diagnóstico mostra a leitora, o ATR, o cartão e o provedor, e o estado do serviço de
      Propagação de Certificados; com o serviço parado (`services.msc`) e o cartão na leitora, o
      aviso dele aparece. Anotar o ATR (se ainda não entrou pelo gate da F2b).
- [ ] Assinar pela página de teste da extensão (`extensao/README.md`, "Testar com o cartão, no
      navegador") no Chrome, no Edge e no Firefox, no Windows 10 e no 11: a assinatura confere
      (`openssl`, ou a própria conferência do programa). PIN errado uma vez (o diálogo do Windows
      diz), e cancelar o diálogo (a página recebe `cancelado`).
- [ ] Com o segundo token (SafeNet ou Gemalto), se veio.
- [ ] Anotar se, sem leitora conectada, o Windows responde `SCARD_E_NO_SERVICE` (é o que o texto do
      diagnóstico presume, pelo serviço Cartão Inteligente que só roda com leitora).
