// Ferramenta de teste manual da F2a: fala native messaging com o programa (o build `dev`), como a
// extensão vai falar, e roda `ola`, `listar`, `conferir` e `assinar` com um bilhete assinado por uma
// chave DEV local. Serve para provar o programa com o cartão de verdade, sem navegador. Não vai
// para release.
//
// Uso, da raiz do repositório (os dois binários vão para `bin/`, fora do versionamento):
//
//	(cd nativo && go build -tags dev -o ../bin/assinador-dev ./cmd/assinador)
//	(cd ferramentas && go build -o ../bin/host-teste ./host-teste)
//	bin/host-teste gerar-chave
//	bin/host-teste listar
//	bin/host-teste assinar --ref <ref do listar> --saida assinatura.bin --certificado certificado.pem
//	bin/host-teste servir      # a página de teste da EXTENSÃO (F3), em http://localhost:8787
//
// `gerar-chave` cria o par ES256 de desenvolvimento: a privada fica em
// `~/.config/confidata-assinador/host-teste-chave.pem` (0600, fora do repositório), e a pública
// entra em `~/.config/confidata-assinador/chaves-dev.json`, que o build `dev` lê.
//
// O PIN é lido do terminal sem eco, vai ao programa dentro do pedido e é zerado em seguida. Ele
// nunca é impresso nem gravado. Um PIN errado encerra: a ferramenta não tenta de novo.
package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	extensaoFirefox  = "assinador@confidata.com.br"
	kidDev           = "dev-host-teste"
	limiteDeResposta = 1024 * 1024
)

func main() {
	if len(os.Args) < 2 {
		uso()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "gerar-chave":
		err = gerarChave()
	case "ola", "listar", "diagnostico":
		err = simples(os.Args[1], os.Args[2:])
	case "assinar":
		err = assinar(os.Args[2:])
	case "servir":
		err = servir(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "uso: host-teste gerar-chave | ola | listar | diagnostico | assinar --ref <hex> [--saida <arquivo>] [--certificado <arquivo.pem>]  (todas aceitam --programa e --origem)")
	fmt.Fprintln(os.Stderr, "     host-teste servir [--porta 8787] [--pagina extensao/e2e/pagina]  (a página de teste da extensão, em http://localhost)")
}

func pastaDeConfiguracao() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "confidata-assinador"), nil
}

// gerarChave cria o par de desenvolvimento e registra a pública no arquivo que o build `dev` lê.
func gerarChave() error {
	dir, err := pastaDeConfiguracao()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(chave)
	if err != nil {
		return err
	}
	caminhoDaPrivada := filepath.Join(dir, "host-teste-chave.pem")
	if err := os.WriteFile(caminhoDaPrivada, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return err
	}
	ponto, err := chave.PublicKey.Bytes()
	if err != nil {
		return err
	}
	type jwk struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	}
	type entrada struct {
		Kid      string `json:"kid"`
		Iss      string `json:"iss"`
		Ambiente string `json:"ambiente"`
		Jwk      jwk    `json:"jwk"`
	}
	caminhoDasPublicas := filepath.Join(dir, "chaves-dev.json")
	var entradas []entrada
	if bruto, err := os.ReadFile(caminhoDasPublicas); err == nil {
		if err := json.Unmarshal(bruto, &entradas); err != nil {
			return fmt.Errorf("%s existe e não é uma lista de chaves: %w", caminhoDasPublicas, err)
		}
	}
	var mantidas []entrada
	for _, e := range entradas {
		if e.Kid != kidDev {
			mantidas = append(mantidas, e)
		}
	}
	mantidas = append(mantidas, entrada{Kid: kidDev, Iss: "confidata", Ambiente: "dev", Jwk: jwk{
		Kty: "EC", Crv: "P-256", X: base64.RawURLEncoding.EncodeToString(ponto[1:33]), Y: base64.RawURLEncoding.EncodeToString(ponto[33:]),
	}})
	saida, _ := json.MarshalIndent(mantidas, "", "  ")
	if err := os.WriteFile(caminhoDasPublicas, append(saida, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Printf("chave privada de desenvolvimento: %s\nchave pública (%s) registrada em %s\n", caminhoDaPrivada, kidDev, caminhoDasPublicas)
	return nil
}

func lerChavePrivada() (*ecdsa.PrivateKey, error) {
	dir, err := pastaDeConfiguracao()
	if err != nil {
		return nil, err
	}
	bruto, err := os.ReadFile(filepath.Join(dir, "host-teste-chave.pem"))
	if err != nil {
		return nil, errors.New("sem chave de desenvolvimento: rode `host-teste gerar-chave`")
	}
	bloco, _ := pem.Decode(bruto)
	if bloco == nil {
		return nil, errors.New("chave de desenvolvimento ilegível")
	}
	k, err := x509.ParsePKCS8PrivateKey(bloco.Bytes)
	if err != nil {
		return nil, err
	}
	chave, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("a chave de desenvolvimento não é ECDSA")
	}
	return chave, nil
}

// programa é o host lançado como o Firefox o lança.
type programa struct {
	cmd    *exec.Cmd
	entra  io.WriteCloser
	sai    io.Reader
	origem string
	seq    int
}

func lancar(caminho, origem string) (*programa, error) {
	cmd := exec.Command(caminho, "/host-teste/br.com.confidata.assinador.json", extensaoFirefox)
	cmd.Stderr = os.Stderr
	entra, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	sai, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("não lancei %s (compile com `cd nativo && go build -tags dev -o ../bin/assinador-dev ./cmd/assinador`): %w", caminho, err)
	}
	return &programa{cmd: cmd, entra: entra, sai: sai, origem: origem}, nil
}

func (p *programa) fechar() {
	p.entra.Close()
	_ = p.cmd.Wait()
}

func escreverQuadro(w io.Writer, corpo []byte) error {
	buf := make([]byte, 4+len(corpo))
	binary.NativeEndian.PutUint32(buf, uint32(len(corpo)))
	copy(buf[4:], corpo)
	_, err := w.Write(buf)
	clear(buf)
	return err
}

func lerQuadro(r io.Reader) ([]byte, error) {
	var cab [4]byte
	if _, err := io.ReadFull(r, cab[:]); err != nil {
		return nil, err
	}
	n := binary.NativeEndian.Uint32(cab[:])
	if n > limiteDeResposta {
		return nil, errors.New("resposta acima do limite")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}

// pedirBruto manda um pedido cujo corpo JSON já está pronto e devolve a resposta decodificada.
func (p *programa) pedirBruto(corpo []byte) (map[string]any, error) {
	if err := escreverQuadro(p.entra, corpo); err != nil {
		return nil, err
	}
	resposta, err := lerQuadro(p.sai)
	if err != nil {
		return nil, err
	}
	var r map[string]any
	return r, json.Unmarshal(resposta, &r)
}

func (p *programa) pedir(op string, dados map[string]string) (map[string]any, error) {
	p.seq++
	m := map[string]any{"v": 1, "id": fmt.Sprintf("host-teste-%d", p.seq), "op": op, "origem": p.origem}
	if dados != nil {
		m["dados"] = dados
	}
	b, _ := json.Marshal(m)
	return p.pedirBruto(b)
}

// pedirComPin monta o JSON do `assinar` com o PIN sem passar o PIN por `string` nem por
// `encoding/json`: os bytes são escapados à mão direto no buffer, que é zerado em seguida.
func (p *programa) pedirComPin(dados map[string]string, pin []byte) (map[string]any, error) {
	p.seq++
	corpo := corpoComPin(fmt.Sprintf("host-teste-%d", p.seq), p.origem, dados, pin)
	r, err := p.pedirBruto(corpo)
	clear(corpo[:cap(corpo)])
	return r, err
}

// corpoComPin monta `{"v":1,"id":…,"op":"assinar","origem":…,"dados":{…,"pin":"…"}}` com o PIN
// escapado direto no buffer, de capacidade fixa (não realoca, e zerá-lo apaga a única cópia).
func corpoComPin(id, origem string, dados map[string]string, pin []byte) []byte {
	cab, _ := json.Marshal(map[string]any{"v": 1, "id": id, "op": "assinar", "origem": origem})
	semPin, _ := json.Marshal(dados)
	corpo := make([]byte, 0, len(cab)+len(semPin)+len(pin)*6+32)
	corpo = append(corpo, cab[:len(cab)-1]...) // sem o `}` final
	corpo = append(corpo, `,"dados":`...)
	corpo = append(corpo, semPin[:len(semPin)-1]...) // sem o `}` final dos dados
	corpo = append(corpo, `,"pin":"`...)
	corpo = escaparJSON(corpo, pin)
	return append(corpo, `"}}`...)
}

// escaparJSON acrescenta `texto` como conteúdo de string JSON.
func escaparJSON(dst, texto []byte) []byte {
	const hexa = "0123456789abcdef"
	for _, c := range texto {
		switch {
		case c == '"' || c == '\\':
			dst = append(dst, '\\', c)
		case c < 0x20:
			dst = append(dst, '\\', 'u', '0', '0', hexa[c>>4], hexa[c&0xf])
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

func opcoes(nome string, args []string) (*flag.FlagSet, *string, *string) {
	fs := flag.NewFlagSet(nome, flag.ExitOnError)
	caminho := fs.String("programa", filepath.Join("bin", "assinador-dev"), "o programa (build dev)")
	origem := fs.String("origem", "http://localhost:3000", "a origem da página (localhost só vale com chave dev)")
	return fs, caminho, origem
}

func mascarar(s string) string {
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

func simples(op string, args []string) error {
	fs, caminho, origem := opcoes(op, args)
	_ = fs.Parse(args)
	p, err := lancar(*caminho, *origem)
	if err != nil {
		return err
	}
	defer p.fechar()
	r, err := p.pedir(op, nil)
	if err != nil {
		return err
	}
	if op != "listar" || r["ok"] != true {
		saida, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(saida))
		return nil
	}
	dados := r["dados"].(map[string]any)
	if len(dados["certificados"].([]any)) == 0 {
		fmt.Println("nenhum certificado (o cartão está na leitora? o módulo dele está no catálogo ou em ~/.config/confidata-assinador/modulos?)")
	}
	for _, c := range dados["certificados"].([]any) {
		cm := c.(map[string]any)
		der, _ := base64.StdEncoding.DecodeString(cm["der"].(string))
		assunto := "(ilegível)"
		if x, err := x509.ParseCertificate(der); err == nil {
			assunto = fmt.Sprintf("%s · emissor %s · válido até %s", mascarar(x.Subject.CommonName), x.Issuer.CommonName, x.NotAfter.Format(time.DateOnly))
		}
		fmt.Printf("ref=%s\n  %s\n  provedor %v (%v) · leitor %v · exige PIN %v · PIN %v\n", cm["ref"], assunto, cm["provedor"], cm["rotuloDoProvedor"], cm["leitor"], cm["exigePin"], cm["estadoDoPin"])
	}
	for _, a := range dados["avisos"].([]any) {
		fmt.Println("aviso:", a)
	}
	return nil
}

// bilhete assina, com a chave dev local, o bilhete que o servidor emitiria para `doc` (o título que a
// janela de confirmação mostra).
func bilhete(chave *ecdsa.PrivateKey, origem, digest, cer, doc string) (string, error) {
	agora := time.Now().Unix()
	cab, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "assinador+jws", "kid": kidDev})
	carga, _ := json.Marshal(map[string]any{"v": 1, "iss": "confidata", "aud": origem, "sid": "host-teste", "dig": digest, "cer": cer,
		"fin": "assinatura", "doc": doc, "org": "Assinador (teste local)", "iat": agora, "exp": agora + 300})
	entrada := base64.RawURLEncoding.EncodeToString(cab) + "." + base64.RawURLEncoding.EncodeToString(carga)
	h := sha256.Sum256([]byte(entrada))
	r, s, err := ecdsa.Sign(rand.Reader, chave, h[:])
	if err != nil {
		return "", err
	}
	a := make([]byte, 64)
	r.FillBytes(a[:32])
	s.FillBytes(a[32:])
	return entrada + "." + base64.RawURLEncoding.EncodeToString(a), nil
}

func assinar(args []string) error {
	fs, caminho, origem := opcoes("assinar", args)
	ref := fs.String("ref", "", "a ref do certificado (sai no listar)")
	saida := fs.String("saida", "", "arquivo da assinatura (binária), para o openssl")
	certSaida := fs.String("certificado", "", "arquivo PEM do certificado, para o openssl")
	_ = fs.Parse(args)
	if len(*ref) != 64 {
		return errors.New("informe --ref (64 caracteres, sai no listar)")
	}
	chave, err := lerChavePrivada()
	if err != nil {
		return err
	}
	p, err := lancar(*caminho, *origem)
	if err != nil {
		return err
	}
	defer p.fechar()

	r, err := p.pedir("listar", nil)
	if err != nil {
		return err
	}
	var escolhido map[string]any
	if r["ok"] == true {
		for _, c := range r["dados"].(map[string]any)["certificados"].([]any) {
			if cm := c.(map[string]any); cm["ref"] == *ref {
				escolhido = cm
			}
		}
	}
	if escolhido == nil {
		return fmt.Errorf("o listar não trouxe a ref %s: %v", *ref, r)
	}
	der, _ := base64.StdEncoding.DecodeString(escolhido["der"].(string))
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}

	resumo := sha256.Sum256([]byte("host-teste " + time.Now().UTC().Format(time.RFC3339Nano)))
	digest := hex.EncodeToString(resumo[:])
	jws, err := bilhete(chave, *origem, digest, *ref, "Teste do Assinador pelo host-teste")
	if err != nil {
		return err
	}
	dados := map[string]string{"ref": *ref, "digest": digest, "bilhete": jws}
	r, err = p.pedir("conferir", dados)
	if err != nil {
		return err
	}
	conferido, _ := json.MarshalIndent(r, "", "  ")
	fmt.Printf("o que a janela de confirmação mostraria:\n%s\n", conferido)
	if r["ok"] != true {
		return errors.New("conferir recusou")
	}

	var pin []byte
	if escolhido["exigePin"] == true {
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return errors.New("o PIN só é lido do terminal")
		}
		fmt.Fprint(os.Stderr, "PIN do cartão: ")
		pin, err = term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		defer clear(pin)
		if len(pin) == 0 {
			return errors.New("PIN vazio: nada foi enviado ao cartão")
		}
		r, err = p.pedirComPin(dados, pin)
		clear(pin)
	} else {
		fmt.Fprintln(os.Stderr, "o dispositivo pede o PIN por conta própria (leitor com teclado ou middleware)")
		r, err = p.pedir("assinar", dados)
	}
	if err != nil {
		return err
	}
	if r["ok"] != true {
		saida, _ := json.MarshalIndent(r, "", "  ")
		return fmt.Errorf("assinar recusou (a ferramenta não tenta de novo):\n%s", saida)
	}
	assinada, err := base64.StdEncoding.DecodeString(r["dados"].(map[string]any)["assinatura"].(string))
	if err != nil {
		return err
	}
	publica, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("o certificado não é RSA")
	}
	if err := rsa.VerifyPKCS1v15(publica, crypto.SHA256, resumo[:], assinada); err != nil {
		return fmt.Errorf("a assinatura NÃO confere com o certificado: %w", err)
	}
	fmt.Println("assinatura confere com a chave pública do certificado (crypto/rsa, PKCS#1 v1.5, SHA-256)")
	if *saida != "" {
		if err := os.WriteFile(*saida, assinada, 0o600); err != nil {
			return err
		}
	}
	if *certSaida != "" {
		if err := os.WriteFile(*certSaida, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
			return err
		}
	}
	if *saida != "" && *certSaida != "" {
		fmt.Printf("conferir no openssl:\n  printf '%%s' %s | xxd -r -p > resumo.bin\n  openssl pkeyutl -verify -certin -inkey %s -in resumo.bin -sigfile %s -pkeyopt digest:sha256\n", digest, *certSaida, *saida)
	}
	return nil
}
