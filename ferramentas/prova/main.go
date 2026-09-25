// Ferramenta de prova da F0: mostra que Go assina um DigestInfo SHA-256 com a chave de um cartão
// ou token A3 por PKCS#11, e mede como cada middleware se comporta (flags de PIN, visibilidade da
// chave antes do login). Não vai para release: o programa de verdade mora em `nativo/`.
//
// Uso:
//
//	prova listar  --modulo /usr/lib/libaetpkss.so.3
//	prova assinar --modulo /usr/lib/libaetpkss.so.3 --ref <sha256 do DER> --digest <sha256 em hex> \
//	              --saida assinatura.bin --certificado certificado.pem
//
// O PIN é lido do terminal sem eco (ou da entrada padrão, quando ela não é terminal, para token de
// teste). Ele nunca é impresso, gravado ou repetido: um PIN errado encerra a operação.
package main

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/miekg/pkcs11"
	"golang.org/x/term"
)

// prefixoDigestInfoSHA256 é o DER de DigestInfo{ sha256, NULL } sem o OCTET STRING do resumo:
// com CKM_RSA_PKCS o cartão assina exatamente o que recebe, então quem monta o DigestInfo é o chamador.
var prefixoDigestInfoSHA256 = []byte{
	0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20,
}

func main() {
	if len(os.Args) < 2 {
		uso()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "listar":
		err = listar(os.Args[2:])
	case "assinar":
		err = assinar(os.Args[2:])
	default:
		uso()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func uso() {
	fmt.Fprintln(os.Stderr, "uso: prova listar --modulo <caminho> | prova assinar --modulo <caminho> --ref <hex> --digest <hex> [--saida <arquivo>] [--certificado <arquivo.pem>]")
}

// DigestInfo monta o bloco que o cartão assina com CKM_RSA_PKCS.
func DigestInfo(digest []byte) ([]byte, error) {
	if len(digest) != sha256.Size {
		return nil, fmt.Errorf("o resumo tem %d bytes; SHA-256 tem %d", len(digest), sha256.Size)
	}
	out := make([]byte, 0, len(prefixoDigestInfoSHA256)+len(digest))
	out = append(out, prefixoDigestInfoSHA256...)
	return append(out, digest...), nil
}

// Mascarar troca os dígitos por asterisco: o CN ICP-Brasil é NOME:CPF, e o CPF não sai da ferramenta.
func Mascarar(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte('*')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Ref é o identificador do certificado no protocolo: SHA-256 do DER, em hexadecimal minúsculo.
func Ref(der []byte) string {
	h := sha256.Sum256(der)
	return hex.EncodeToString(h[:])
}

type certificadoNoToken struct {
	slot   uint
	id     []byte
	rotulo string
	der    []byte
	cert   *x509.Certificate
}

func abrir(modulo string) (*pkcs11.Ctx, error) {
	if modulo == "" {
		return nil, errors.New("informe --modulo")
	}
	p := pkcs11.New(modulo)
	if p == nil {
		return nil, fmt.Errorf("o módulo %s não carregou", modulo)
	}
	if err := p.Initialize(); err != nil {
		p.Destroy()
		return nil, fmt.Errorf("C_Initialize: %w", err)
	}
	return p, nil
}

func fechar(p *pkcs11.Ctx) {
	_ = p.Finalize()
	p.Destroy()
}

func flagsDoToken(f uint) string {
	partes := []string{}
	add := func(bit uint, nome string) {
		if f&bit != 0 {
			partes = append(partes, nome)
		}
	}
	add(pkcs11.CKF_LOGIN_REQUIRED, "LOGIN_REQUIRED")
	add(pkcs11.CKF_PROTECTED_AUTHENTICATION_PATH, "PROTECTED_AUTHENTICATION_PATH")
	add(pkcs11.CKF_USER_PIN_COUNT_LOW, "USER_PIN_COUNT_LOW")
	add(pkcs11.CKF_USER_PIN_FINAL_TRY, "USER_PIN_FINAL_TRY")
	add(pkcs11.CKF_USER_PIN_LOCKED, "USER_PIN_LOCKED")
	add(pkcs11.CKF_TOKEN_INITIALIZED, "TOKEN_INITIALIZED")
	add(pkcs11.CKF_USER_PIN_INITIALIZED, "USER_PIN_INITIALIZED")
	if len(partes) == 0 {
		return "(nenhuma)"
	}
	return strings.Join(partes, ", ")
}

func certificadosDoSlot(p *pkcs11.Ctx, sessao pkcs11.SessionHandle, slot uint) ([]certificadoNoToken, error) {
	if err := p.FindObjectsInit(sessao, []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE)}); err != nil {
		return nil, fmt.Errorf("C_FindObjectsInit: %w", err)
	}
	objetos, _, err := p.FindObjects(sessao, 64)
	_ = p.FindObjectsFinal(sessao)
	if err != nil {
		return nil, fmt.Errorf("C_FindObjects: %w", err)
	}
	var out []certificadoNoToken
	for _, o := range objetos {
		attrs, err := p.GetAttributeValue(sessao, o, []*pkcs11.Attribute{
			pkcs11.NewAttribute(pkcs11.CKA_VALUE, nil),
			pkcs11.NewAttribute(pkcs11.CKA_ID, nil),
			pkcs11.NewAttribute(pkcs11.CKA_LABEL, nil),
		})
		if err != nil {
			continue
		}
		c := certificadoNoToken{slot: slot, der: attrs[0].Value, id: attrs[1].Value, rotulo: string(attrs[2].Value)}
		if cert, err := x509.ParseCertificate(c.der); err == nil {
			c.cert = cert
		}
		out = append(out, c)
	}
	return out, nil
}

func chavePrivada(p *pkcs11.Ctx, sessao pkcs11.SessionHandle, id []byte) (pkcs11.ObjectHandle, bool, error) {
	if err := p.FindObjectsInit(sessao, []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_ID, id),
	}); err != nil {
		return 0, false, fmt.Errorf("C_FindObjectsInit: %w", err)
	}
	objetos, _, err := p.FindObjects(sessao, 2)
	_ = p.FindObjectsFinal(sessao)
	if err != nil {
		return 0, false, fmt.Errorf("C_FindObjects: %w", err)
	}
	if len(objetos) == 0 {
		return 0, false, nil
	}
	return objetos[0], true, nil
}

func usoDaChave(c *x509.Certificate) string {
	if c == nil {
		return "?"
	}
	partes := []string{}
	if c.KeyUsage&x509.KeyUsageDigitalSignature != 0 {
		partes = append(partes, "digitalSignature")
	}
	if c.KeyUsage&x509.KeyUsageContentCommitment != 0 {
		partes = append(partes, "nonRepudiation")
	}
	if len(partes) == 0 {
		return "(sem uso de assinatura)"
	}
	return strings.Join(partes, ", ")
}

func listar(args []string) error {
	fs := flag.NewFlagSet("listar", flag.ContinueOnError)
	modulo := fs.String("modulo", "", "caminho do módulo PKCS#11")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := abrir(*modulo)
	if err != nil {
		return err
	}
	defer fechar(p)

	if info, err := p.GetInfo(); err == nil {
		fmt.Printf("módulo: %s · %s · Cryptoki %d.%d · biblioteca %d.%d\n", *modulo, strings.TrimSpace(info.ManufacturerID),
			info.CryptokiVersion.Major, info.CryptokiVersion.Minor, info.LibraryVersion.Major, info.LibraryVersion.Minor)
	}
	slots, err := p.GetSlotList(true)
	if err != nil {
		return fmt.Errorf("C_GetSlotList: %w", err)
	}
	if len(slots) == 0 {
		fmt.Println("nenhum slot com token presente (a leitora está conectada e o cartão inserido?)")
		return nil
	}
	for _, slot := range slots {
		fmt.Printf("\nslot %d\n", slot)
		if si, err := p.GetSlotInfo(slot); err == nil {
			fmt.Printf("  leitora: %s\n", strings.TrimSpace(si.SlotDescription))
		}
		ti, err := p.GetTokenInfo(slot)
		if err != nil {
			fmt.Printf("  C_GetTokenInfo falhou: %v\n", err)
			continue
		}
		fmt.Printf("  token: %s · fabricante %s · modelo %s\n", strings.TrimSpace(ti.Label), strings.TrimSpace(ti.ManufacturerID), strings.TrimSpace(ti.Model))
		fmt.Printf("  flags: %s\n", flagsDoToken(ti.Flags))
		fmt.Printf("  medição (a): caminho protegido de autenticação = %v\n", ti.Flags&pkcs11.CKF_PROTECTED_AUTHENTICATION_PATH != 0)
		fmt.Printf("  PIN: mínimo %d, máximo %d\n", ti.MinPinLen, ti.MaxPinLen)

		sessao, err := p.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION)
		if err != nil {
			fmt.Printf("  C_OpenSession falhou: %v\n", err)
			continue
		}
		certs, err := certificadosDoSlot(p, sessao, slot)
		if err != nil {
			fmt.Printf("  %v\n", err)
		}
		for _, c := range certs {
			fmt.Printf("  certificado ref=%s\n", Ref(c.der))
			if c.cert != nil {
				fmt.Printf("    assunto: %s\n", Mascarar(c.cert.Subject.CommonName))
				fmt.Printf("    emissor: %s\n", c.cert.Issuer.CommonName)
				fmt.Printf("    válido até: %s · uso: %s · chave: %s\n", c.cert.NotAfter.Format(time.DateOnly), usoDaChave(c.cert), c.cert.PublicKeyAlgorithm)
			} else {
				fmt.Printf("    (DER não reconhecido como X.509)\n")
			}
			_, visivel, _ := chavePrivada(p, sessao, c.id)
			fmt.Printf("    chave privada visível sem login: %v\n", visivel)
		}
		_ = p.CloseSession(sessao)
	}
	return nil
}

// lerPin devolve o PIN em []byte. Do terminal, sem eco; fora do terminal, a primeira linha da
// entrada padrão (token de teste). Quem chama zera o slice depois do login.
func lerPin() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "PIN do cartão: ")
		pin, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return pin, err
	}
	linha, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil && len(linha) == 0 {
		return nil, errors.New("sem PIN na entrada padrão")
	}
	return bytes.TrimRight(linha, "\r\n"), nil
}

func zerar(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func assinar(args []string) error {
	fs := flag.NewFlagSet("assinar", flag.ContinueOnError)
	modulo := fs.String("modulo", "", "caminho do módulo PKCS#11")
	ref := fs.String("ref", "", "SHA-256 em hexadecimal do DER do certificado (sai no listar)")
	digestHex := fs.String("digest", "", "resumo SHA-256 em hexadecimal (64 caracteres)")
	saida := fs.String("saida", "", "arquivo onde gravar a assinatura (binária), para conferir no openssl")
	certSaida := fs.String("certificado", "", "arquivo PEM onde gravar o certificado, para conferir no openssl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	digest, err := hex.DecodeString(strings.ToLower(*digestHex))
	if err != nil || len(digest) != sha256.Size {
		return errors.New("--digest precisa ser o SHA-256 em hexadecimal (64 caracteres)")
	}
	bloco, err := DigestInfo(digest)
	if err != nil {
		return err
	}

	p, err := abrir(*modulo)
	if err != nil {
		return err
	}
	defer fechar(p)

	slots, err := p.GetSlotList(true)
	if err != nil {
		return fmt.Errorf("C_GetSlotList: %w", err)
	}
	var alvo *certificadoNoToken
	var sessao pkcs11.SessionHandle
	for _, slot := range slots {
		s, err := p.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION)
		if err != nil {
			continue
		}
		certs, _ := certificadosDoSlot(p, s, slot)
		for i := range certs {
			if Ref(certs[i].der) == strings.ToLower(*ref) {
				alvo = &certs[i]
				break
			}
		}
		if alvo != nil {
			sessao = s
			break
		}
		_ = p.CloseSession(s)
	}
	if alvo == nil {
		return errors.New("certificado-nao-encontrado: nenhum certificado com essa ref nos tokens presentes")
	}
	defer func() { _ = p.CloseSession(sessao) }()
	if alvo.cert == nil {
		return errors.New("o certificado do token não é X.509 legível")
	}
	pub, ok := alvo.cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("algoritmo-nao-suportado: só RSA na v1")
	}
	if time.Now().After(alvo.cert.NotAfter) {
		return fmt.Errorf("o certificado venceu em %s: é listado, nunca assinado", alvo.cert.NotAfter.Format(time.DateOnly))
	}

	ti, err := p.GetTokenInfo(alvo.slot)
	if err != nil {
		return fmt.Errorf("C_GetTokenInfo: %w", err)
	}
	protegido := ti.Flags&pkcs11.CKF_PROTECTED_AUTHENTICATION_PATH != 0
	var pin []byte
	if protegido {
		fmt.Fprintln(os.Stderr, "o token tem caminho protegido de autenticação: o PIN é pedido pelo leitor ou pelo middleware")
	} else {
		pin, err = lerPin()
		if err != nil {
			return err
		}
	}
	inicio := time.Now()
	// O Login do pacote recebe string. A string abaixo aponta para os MESMOS bytes do slice, sem
	// cópia em Go; zerar o slice a apaga. A cópia que o pacote faz para o C (C.CString) não é zerada
	// por ele: é achado para a F2a, registrado nas medições da F0.
	err = p.Login(sessao, pkcs11.CKU_USER, unsafe.String(unsafe.SliceData(pin), len(pin)))
	zerar(pin)
	if err != nil {
		var e pkcs11.Error
		if errors.As(err, &e) {
			switch e {
			case pkcs11.CKR_PIN_INCORRECT:
				return errors.New("pin-incorreto (a ferramenta não tenta de novo)")
			case pkcs11.CKR_PIN_LOCKED:
				return errors.New("token-bloqueado")
			case pkcs11.CKR_USER_ALREADY_LOGGED_IN:
				// segue: outra aplicação já abriu a sessão de usuário neste token
			default:
				return fmt.Errorf("C_Login: %w", err)
			}
		} else {
			return fmt.Errorf("C_Login: %w", err)
		}
	}
	defer func() { _ = p.Logout(sessao) }()
	fmt.Fprintf(os.Stderr, "login em %s\n", time.Since(inicio).Round(time.Millisecond))

	chave, achou, err := chavePrivada(p, sessao, alvo.id)
	if err != nil {
		return err
	}
	if !achou {
		return errors.New("chave-ausente: o token não tem chave privada com o CKA_ID do certificado")
	}
	if err := p.SignInit(sessao, []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS, nil)}, chave); err != nil {
		return fmt.Errorf("C_SignInit: %w", err)
	}
	inicio = time.Now()
	assinatura, err := p.Sign(sessao, bloco)
	if err != nil {
		return fmt.Errorf("C_Sign: %w", err)
	}
	fmt.Fprintf(os.Stderr, "assinatura em %s, %d bytes\n", time.Since(inicio).Round(time.Millisecond), len(assinatura))

	// Conferência local, independente do cartão.
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest, assinatura); err != nil {
		return fmt.Errorf("a assinatura NÃO confere com a chave pública do certificado: %w", err)
	}
	fmt.Println("assinatura confere com a chave pública do certificado (crypto/rsa, PKCS#1 v1.5, SHA-256)")

	if *saida != "" {
		if err := os.WriteFile(*saida, assinatura, 0o600); err != nil {
			return err
		}
		fmt.Printf("assinatura gravada em %s\n", *saida)
	}
	if *certSaida != "" {
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: alvo.der})
		if err := os.WriteFile(*certSaida, pemBytes, 0o600); err != nil {
			return err
		}
		fmt.Printf("certificado gravado em %s\n", *certSaida)
	}
	if *saida != "" && *certSaida != "" {
		fmt.Printf("conferir no openssl:\n  printf '%%s' %s | xxd -r -p > resumo.bin\n  openssl pkeyutl -verify -certin -inkey %s -in resumo.bin -sigfile %s -pkeyopt digest:sha256\n", hex.EncodeToString(digest), *certSaida, *saida)
	}
	return nil
}
