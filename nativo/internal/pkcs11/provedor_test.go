//go:build !windows

package pkcs11

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	p11 "github.com/miekg/pkcs11"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
	"github.com/Gottbrok/assinador/nativo/internal/softhsmteste"
)

// O binário de teste faz o papel do programa: relançado como `<teste> modulo --caminho X`, ele é
// o filho de um módulo.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "modulo" && os.Args[2] == "--caminho" {
		os.Exit(ExecutarModulo(os.Args[3]))
	}
	os.Exit(m.Run())
}

func provedorDe(t *testing.T, modulos ...Modulo) *Provedor {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Provedor{Modulos: modulos, Executavel: exe, PrazoDeListar: 20 * time.Second, PrazoDeAssinar: 20 * time.Second}
}

func moduloSoftHSM(tk *softhsmteste.Token) Modulo {
	return Modulo{Caminho: tk.Modulo, Nome: "softhsm", Rotulo: "SoftHSM", Origem: OrigemConfiguracao}
}

func refDe(tk *softhsmteste.Token, nome string) string {
	return assinatura.Ref(tk.Certificados[nome].Raw)
}

func porRef(certs []assinatura.Certificado, ref string) (assinatura.Certificado, bool) {
	i := slices.IndexFunc(certs, func(c assinatura.Certificado) bool { return c.Ref == ref })
	if i < 0 {
		return assinatura.Certificado{}, false
	}
	return certs[i], true
}

func erroDoProtocolo(t *testing.T, err error) *protocolo.Erro {
	t.Helper()
	var e *protocolo.Erro
	if !errors.As(err, &e) {
		t.Fatalf("erro fora do protocolo: %v", err)
	}
	return e
}

// O C_Login próprio entra com o PIN certo, recusa o errado, e confirma a cópia zerada nos dois
// casos; e a lista de funções tem o desenho do padrão nesta plataforma.
func TestLoginProprioZeraACopia(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	ctx := p11.New(tk.Modulo)
	if err := ctx.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ctx.Finalize()
		ctx.Destroy()
	}()
	if !listaCoerente(tk.Modulo) {
		t.Fatal("a lista de funções não tem C_GetFunctionList na posição do padrão")
	}
	slots, _ := ctx.GetSlotList(true)
	s, err := ctx.OpenSession(slots[0], p11.CKF_SERIAL_SESSION)
	if err != nil {
		t.Fatal(err)
	}
	pin := []byte(softhsmteste.PIN)
	zerado, err := entrarNoToken(tk.Modulo, s, usuarioComum, pin)
	if err != nil || !zerado {
		t.Fatalf("PIN certo: zerado=%v %v", zerado, err)
	}
	if string(pin) != softhsmteste.PIN {
		t.Fatal("o login mexeu no PIN de quem chamou (ele pode precisar de um segundo login)")
	}
	_ = ctx.Logout(s)
	zerado, err = entrarNoToken(tk.Modulo, s, usuarioComum, []byte("000000"))
	if e, ok := codigoCkr(err); !ok || e != p11.CKR_PIN_INCORRECT || !zerado {
		t.Fatalf("PIN errado: zerado=%v %v", zerado, err)
	}
	if _, err := entrarNoToken("/nao/carregado.so", s, usuarioComum, pin); err == nil {
		t.Fatal("entrou por módulo não carregado (o dlopen com RTLD_NOLOAD não pode carregar outra cópia)")
	}
}

// Com as chaves escondidas até o login (o comum em cartão), a lista traz todo certificado do
// token; quem tira a AC e a chave EC é o host (`assinatura.Listavel`), e `assinar` confere a chave.
func TestListarPeloFilhoComChavesEscondidas(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	certs, avisos := provedorDe(t, moduloSoftHSM(tk)).Listar(context.Background())
	if len(avisos) != 0 {
		t.Fatalf("avisos: %v", avisos)
	}
	if len(certs) != 5 {
		t.Fatalf("esperava 5 certificados, veio %d", len(certs))
	}
	c, ok := porRef(certs, refDe(tk, softhsmteste.Titular))
	if !ok {
		t.Fatal("o titular não veio")
	}
	if c.Provedor != "pkcs11:softhsm" || c.RotuloDoProvedor != "SoftHSM" || !c.ExigePin || c.EstadoDoPin != protocolo.PinOk || c.ChaveConfirmada || c.Leitor == "" {
		t.Fatalf("certificado: %+v", c)
	}
	if !bytes.Equal(c.DER, tk.Certificados[softhsmteste.Titular].Raw) {
		t.Fatal("DER diferente do gravado")
	}
}

// Num token que mostra umas chaves antes do login e esconde outras, a chave vista vem confirmada,
// e o certificado da chave escondida (e o da AC, que o host tira) vem sem confirmação, mas vem:
// escondê-lo impediria a pessoa de assinar com ele.
func TestListarPeloFilhoComChavesVisiveis(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{ChavesVisiveis: true})
	certs, _ := provedorDe(t, moduloSoftHSM(tk)).Listar(context.Background())
	if len(certs) != 5 {
		t.Fatalf("esperava 5 certificados, veio %d", len(certs))
	}
	confirmadas := map[string]bool{softhsmteste.Titular: true, softhsmteste.Vencido: true, softhsmteste.Eliptica: true, softhsmteste.AC: false, softhsmteste.SempreAutentica: false}
	for nome, quer := range confirmadas {
		c, ok := porRef(certs, refDe(tk, nome))
		if !ok || c.ChaveConfirmada != quer {
			t.Errorf("%s: listado=%v confirmada=%v", nome, ok, c.ChaveConfirmada)
		}
	}
}

func assinarCom(t *testing.T, p *Provedor, certs []assinatura.Certificado, ref string, pin string) ([]byte, [32]byte, error) {
	t.Helper()
	c, ok := porRef(certs, ref)
	if !ok {
		t.Fatalf("certificado %s não listado", ref)
	}
	digest := sha256.Sum256([]byte("documento de teste " + ref))
	var bytesDoPin []byte
	if pin != "" {
		bytesDoPin = []byte(pin)
	}
	assinada, err := p.Assinar(context.Background(), c, digest, bytesDoPin)
	if bytesDoPin != nil && !bytes.Equal(bytesDoPin, make([]byte, len(bytesDoPin))) {
		t.Fatal("o provedor não zerou o PIN que recebeu")
	}
	return assinada, digest, err
}

func TestAssinarPeloFilho(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	p := provedorDe(t, moduloSoftHSM(tk))
	certs, _ := p.Listar(context.Background())

	assinada, digest, err := assinarCom(t, p, certs, refDe(tk, softhsmteste.Titular), softhsmteste.PIN)
	if err != nil {
		t.Fatal(err)
	}
	pub := tk.Certificados[softhsmteste.Titular].PublicKey.(*rsa.PublicKey)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], assinada); err != nil {
		t.Fatalf("a assinatura não confere: %v", err)
	}
	conferirComOpenssl(t, tk.Certificados[softhsmteste.Titular].Raw, digest, assinada)

	// Chave com CKA_ALWAYS_AUTHENTICATE: o login da operação vai com o mesmo PIN.
	assinada, digest, err = assinarCom(t, p, certs, refDe(tk, softhsmteste.SempreAutentica), softhsmteste.PIN)
	if err != nil {
		t.Fatalf("sempre autentica: %v", err)
	}
	pub = tk.Certificados[softhsmteste.SempreAutentica].PublicKey.(*rsa.PublicKey)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], assinada); err != nil {
		t.Fatalf("sempre autentica não confere: %v", err)
	}
}

// A assinatura também confere pelo openssl, que é como a F0 e o gate da F2a conferem a do cartão.
func conferirComOpenssl(t *testing.T, der []byte, digest [32]byte, assinada []byte) {
	t.Helper()
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Log("sem openssl: conferência só pelo crypto/rsa")
		return
	}
	dir := t.TempDir()
	certificado := filepath.Join(dir, "c.der")
	resumo := filepath.Join(dir, "r.bin")
	arquivo := filepath.Join(dir, "a.bin")
	for caminho, conteudo := range map[string][]byte{certificado: der, resumo: digest[:], arquivo: assinada} {
		if err := os.WriteFile(caminho, conteudo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pem := filepath.Join(dir, "c.pem")
	if out, err := exec.Command(openssl, "x509", "-inform", "DER", "-in", certificado, "-out", pem).CombinedOutput(); err != nil {
		t.Fatalf("openssl x509: %v %s", err, out)
	}
	out, err := exec.Command(openssl, "pkeyutl", "-verify", "-certin", "-inkey", pem, "-in", resumo, "-sigfile", arquivo, "-pkeyopt", "digest:sha256").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Signature Verified Successfully") {
		t.Fatalf("openssl pkeyutl -verify: %v %s", err, out)
	}
}

// O SoftHSM 2.6 marca CKF_USER_PIN_COUNT_LOW depois de um PIN errado, guarda a marca entre
// processos e a limpa no login certo, mas nunca bloqueia (medido em 2026-09-25, dez tentativas).
// É o que prova, de ponta a ponta, a releitura das flags DEPOIS da falha; o bloqueio e a última
// tentativa se provam pelo mapa, em `erros_test.go`.
func TestAssinarComPinErradoNaoRepete(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	p := provedorDe(t, moduloSoftHSM(tk))
	certs, _ := p.Listar(context.Background())
	_, _, err := assinarCom(t, p, certs, refDe(tk, softhsmteste.Titular), "000000")
	if e := erroDoProtocolo(t, err); e.Codigo != protocolo.PinIncorreto || e.Tentativas != protocolo.TentativasPoucas {
		t.Fatalf("PIN errado: %+v", e)
	}
	depois, _ := p.Listar(context.Background())
	if c, _ := porRef(depois, refDe(tk, softhsmteste.Titular)); c.EstadoDoPin != protocolo.PinPoucasTentativas {
		t.Fatalf("a lista não mostrou as poucas tentativas: %q", c.EstadoDoPin)
	}
	// Depois do erro, o PIN certo ainda entra: o programa não gastou tentativas sozinho.
	if _, _, err := assinarCom(t, p, certs, refDe(tk, softhsmteste.Titular), softhsmteste.PIN); err != nil {
		t.Fatalf("PIN certo depois do errado: %v", err)
	}
	depois, _ = p.Listar(context.Background())
	if c, _ := porRef(depois, refDe(tk, softhsmteste.Titular)); c.EstadoDoPin != protocolo.PinOk {
		t.Fatalf("o login certo não limpou o estado: %q", c.EstadoDoPin)
	}
}

func TestAssinarSemPinNaoTocaOCartao(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	p := provedorDe(t, moduloSoftHSM(tk))
	certs, _ := p.Listar(context.Background())
	_, _, err := assinarCom(t, p, certs, refDe(tk, softhsmteste.Titular), "")
	if e := erroDoProtocolo(t, err); e.Codigo != protocolo.Protocolo {
		t.Fatalf("sem PIN: %v", e)
	}
}

func TestAssinarSemChaveEhChaveAusente(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	p := provedorDe(t, moduloSoftHSM(tk))
	certs, _ := p.Listar(context.Background())
	_, _, err := assinarCom(t, p, certs, refDe(tk, softhsmteste.AC), softhsmteste.PIN)
	if e := erroDoProtocolo(t, err); e.Codigo != protocolo.ChaveAusente {
		t.Fatalf("AC: %v", e)
	}
	_, _, err = assinarCom(t, p, certs, refDe(tk, softhsmteste.Eliptica), softhsmteste.PIN)
	if e := erroDoProtocolo(t, err); e.Codigo != protocolo.AlgoritmoNaoSuportado {
		t.Fatalf("EC: %v", e)
	}
}

func compilarModuloDeTeste(t *testing.T) string {
	t.Helper()
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("sem gcc")
	}
	saida := filepath.Join(t.TempDir(), "libmodulo-que-cai.so")
	fonte := filepath.Join("..", "..", "testes", "modulo-que-cai", "modulo.c")
	if out, err := exec.Command(gcc, "-shared", "-fPIC", "-o", saida, fonte).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v %s", err, out)
	}
	return saida
}

// Um módulo que aborta no C_Initialize derruba só o SEU filho: os certificados do outro chegam, e
// o que caiu vira aviso e `falhou` no diagnóstico.
func TestModuloQueCaiDerrubaSoOFilho(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	cai := Modulo{Caminho: compilarModuloDeTeste(t), Nome: "cai", Rotulo: "Módulo que cai"}
	p := provedorDe(t, cai, moduloSoftHSM(tk))
	certs, avisos := p.Listar(context.Background())
	if len(certs) != 5 {
		t.Fatalf("os certificados do SoftHSM não chegaram: %d", len(certs))
	}
	if len(avisos) != 1 || !strings.Contains(avisos[0], "Módulo que cai") {
		t.Fatalf("avisos: %v", avisos)
	}
	diag := p.Diagnosticar(context.Background())
	if len(diag) != 2 || diag[0].Estado != assinatura.EstadoFalhou || diag[1].Estado != assinatura.EstadoCarregado || diag[1].Certificados != 5 {
		t.Fatalf("diagnóstico: %+v", diag)
	}
}

// Um módulo que trava vira aviso no prazo, e não segura a lista.
func TestModuloQueTravaRespeitaOPrazo(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	t.Setenv("ASSINADOR_MODULO_DE_TESTE", "trava")
	trava := Modulo{Caminho: compilarModuloDeTeste(t), Nome: "trava", Rotulo: "Módulo que trava"}
	p := provedorDe(t, trava, moduloSoftHSM(tk))
	p.PrazoDeListar = 2 * time.Second
	inicio := time.Now()
	certs, avisos := p.Listar(context.Background())
	if d := time.Since(inicio); d > 6*time.Second {
		t.Fatalf("a lista esperou %s", d)
	}
	if len(certs) != 5 || len(avisos) != 1 {
		t.Fatalf("certs %d avisos %v", len(certs), avisos)
	}
	// E no assinar, o prazo é `tempo-esgotado`.
	p.PrazoDeAssinar = time.Second
	_, err := p.Assinar(context.Background(), assinatura.Certificado{Ref: strings.Repeat("a", 64), Interno: trava}, [32]byte{}, nil)
	if e := erroDoProtocolo(t, err); e.Codigo != protocolo.TempoEsgotado {
		t.Fatalf("assinar travado: %v", e)
	}
}

// Um módulo que trava ao ENCERRAR (C_Logout, C_CloseSession, C_Finalize ou o dlclose) depois de ter
// feito o trabalho não pode custar a resposta: o filho responde ANTES de limpar, e o pai o mata
// depois de `esperaDoFim`. Sem isso, um cartão que já assinou virava `tempo-esgotado` em toda
// tentativa.
func TestModuloQueTravaNoFimNaoSeguraAResposta(t *testing.T) {
	t.Setenv("ASSINADOR_MODULO_DE_TESTE", "trava-no-fim")
	m := Modulo{Caminho: compilarModuloDeTeste(t), Nome: "trava-no-fim", Rotulo: "Módulo que trava no fim"}
	p := provedorDe(t, m)
	p.PrazoDeListar = 10 * time.Second
	inicio := time.Now()
	certs, avisos := p.Listar(context.Background())
	if d := time.Since(inicio); d > esperaDoFim+2*time.Second {
		t.Fatalf("a resposta esperou a limpeza travada: %s", d)
	}
	if len(certs) != 0 || len(avisos) != 0 {
		t.Fatalf("o módulo respondeu e mesmo assim falhou: certs %d, avisos %v", len(certs), avisos)
	}
}

// Dois módulos vendo o mesmo cartão (o SafeSign e o OpenSC): a lista funde por `ref` e fica com o
// do fabricante, venha ele antes ou depois do genérico.
func TestFusaoPorRefPrefereOFabricante(t *testing.T) {
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	copia := filepath.Join(t.TempDir(), "libsofthsm2-copia.so")
	origem, err := os.Open(tk.Modulo)
	if err != nil {
		t.Fatal(err)
	}
	destino, err := os.Create(copia)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(destino, origem); err != nil {
		t.Fatal(err)
	}
	origem.Close()
	destino.Close()
	generico := Modulo{Caminho: copia, Nome: "generico", Rotulo: "Genérico", Generico: true}
	fabricante := moduloSoftHSM(tk)
	for _, ordem := range [][]Modulo{{generico, fabricante}, {fabricante, generico}} {
		certs, avisos := provedorDe(t, ordem...).Listar(context.Background())
		if len(certs) != 5 || len(avisos) != 0 {
			t.Fatalf("fusão: %d certificados, avisos %v", len(certs), avisos)
		}
		for _, c := range certs {
			if c.Provedor != "pkcs11:softhsm" {
				t.Fatalf("ficou com o genérico: %s", c.Provedor)
			}
		}
	}
}
