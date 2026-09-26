//go:build windows

package windows

import (
	"bytes"
	"context"
	"runtime"
	"sort"
	"unsafe"

	win "golang.org/x/sys/windows"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Aviso da lista, em frase para a pessoa.
const AvisoRepositorio = "O repositório de certificados do Windows não abriu, e a lista pode estar incompleta."

// NomeDoRepositorio é como o diagnóstico chama o repositório do usuário quando nenhum certificado
// com chave foi achado nele.
const NomeDoRepositorio = "Windows (repositório do usuário)"

// Provedor lê o repositório pessoal do usuário e assina pelo CNG ou pelo CSP.
type Provedor struct {
	// JanelaMae é a janela que o Chrome informa (`--parent-window`): o diálogo de PIN do provedor
	// abre na frente dela. Zero quando o navegador não informa (o Firefox).
	JanelaMae uint64
	// Repositorio é o repositório do sistema que o provedor lê ("MY", o pessoal). O teste usa outro.
	Repositorio string
}

// NovoProvedor é o provedor do repositório pessoal (`CurrentUser\My`).
func NovoProvedor(janela uint64) *Provedor {
	return &Provedor{JanelaMae: janela, Repositorio: "MY"}
}

// visto é um certificado do repositório com chave privada associada.
type visto struct {
	der      []byte
	provedor string
	tipo     uint32
}

// abrirRepositorio abre o repositório do usuário só para leitura.
func (p *Provedor) abrirRepositorio() (win.Handle, error) {
	nome, err := win.UTF16PtrFromString(p.Repositorio)
	if err != nil {
		return 0, err
	}
	return win.CertOpenStore(win.CERT_STORE_PROV_SYSTEM, 0, 0, win.CERT_SYSTEM_STORE_CURRENT_USER|win.CERT_STORE_READONLY_FLAG|win.CERT_STORE_OPEN_EXISTING_FLAG, uintptr(unsafe.Pointer(nome)))
}

// percorrer chama `f` para cada certificado do repositório que tem chave privada associada, até `f`
// devolver `false` (e então entrega o contexto dele, que quem chamou libera).
func percorrer(repositorio win.Handle, f func(ctx *win.CertContext, v visto) bool) *win.CertContext {
	var ctx *win.CertContext
	for {
		ctx, _ = win.CertEnumCertificatesInStore(repositorio, ctx)
		if ctx == nil {
			return nil
		}
		nome, tipo, ok := provedorDaChave(ctx)
		if !ok || ctx.Length == 0 {
			continue
		}
		der := bytes.Clone(unsafe.Slice(ctx.EncodedCert, ctx.Length))
		if !f(ctx, visto{der: der, provedor: nome, tipo: tipo}) {
			return ctx
		}
	}
}

// vistos são os certificados com chave do repositório.
func (p *Provedor) vistos() ([]visto, error) {
	repositorio, err := p.abrirRepositorio()
	if err != nil {
		return nil, err
	}
	defer win.CertCloseStore(repositorio, 0)
	var saida []visto
	percorrer(repositorio, func(_ *win.CertContext, v visto) bool {
		saida = append(saida, v)
		return true
	})
	return saida, nil
}

func certificadoDe(v visto) assinatura.Certificado {
	return assinatura.Certificado{
		Ref:              assinatura.Ref(v.der),
		DER:              v.der,
		Provedor:         "windows:" + caminhoDoTipo(v.tipo),
		RotuloDoProvedor: rotuloDoProvedor(v.provedor),
		// O PIN é pedido pelo provedor, num diálogo do Windows (§3.5 do plano).
		ExigePin:        false,
		ChaveConfirmada: true,
	}
}

// Listar lê o repositório sem abrir chave nenhuma: listar nunca pede PIN. Quem filtra o que entra na
// lista (AC, uso, algoritmo) é o host, pela regra comum a todo provedor.
func (p *Provedor) Listar(_ context.Context) ([]assinatura.Certificado, []string) {
	vs, err := p.vistos()
	if err != nil {
		return nil, []string{AvisoRepositorio}
	}
	certificados := make([]assinatura.Certificado, 0, len(vs))
	for _, v := range vs {
		certificados = append(certificados, certificadoDe(v))
	}
	return certificados, nil
}

// resultado de uma assinatura no Windows.
type resultado struct {
	assinatura []byte
	erro       *protocolo.Erro
}

// Assinar acha o certificado de novo no repositório, abre a chave dele e assina. O PIN, se houver, é
// pedido pelo provedor num diálogo do Windows, e o que veio da extensão (nada, com `exigePin` falso)
// é zerado. A chamada ao provedor BLOQUEIA enquanto o diálogo está aberto e não se interrompe: ela
// corre à parte, e o cancelamento (a extensão que fechou o canal) responde na hora; o processo sai e
// leva o diálogo junto.
func (p *Provedor) Assinar(ctx context.Context, c assinatura.Certificado, digest [32]byte, pin []byte) ([]byte, error) {
	assinatura.Zerar(pin)
	pronto := make(chan resultado, 1)
	go func() {
		// A chave e o diálogo dela ficam numa thread só do começo ao fim: há CSP que guarda estado
		// por thread.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		a, e := p.assinarNoWindows(c.DER, digest)
		pronto <- resultado{assinatura: a, erro: e}
	}()
	select {
	case r := <-pronto:
		if r.erro != nil {
			return nil, r.erro
		}
		return r.assinatura, nil
	case <-ctx.Done():
		return nil, protocolo.Novo(protocolo.Cancelado, "o canal com a extensão fechou durante a assinatura")
	}
}

func (p *Provedor) assinarNoWindows(der []byte, digest [32]byte) ([]byte, *protocolo.Erro) {
	repositorio, err := p.abrirRepositorio()
	if err != nil {
		return nil, erroDoWindows("abrir o repositório", codigoDoErro(err))
	}
	defer win.CertCloseStore(repositorio, 0)
	ctx := percorrer(repositorio, func(_ *win.CertContext, v visto) bool { return !bytes.Equal(v.der, der) })
	if ctx == nil {
		return nil, protocolo.Novo(protocolo.CertificadoNaoEncontrado, "o certificado saiu do repositório do Windows")
	}
	defer win.CertFreeCertificateContext(ctx)
	assinada, _, e := p.assinarComContexto(ctx, digest, true)
	return assinada, e
}

// assinarComContexto abre a chave do certificado e assina, e diz por qual caminho (CaminhoCNG ou
// CaminhoCSP). `preferirCNG` é o caminho do programa: o CNG quando o provedor o oferece (os KSP, e os
// CSP da Microsoft, que o CNG também abre); o teste o desliga para exercitar o caminho do CSP
// legado, que é o de muitos cartões.
func (p *Provedor) assinarComContexto(ctx *win.CertContext, digest [32]byte, preferirCNG bool) ([]byte, string, *protocolo.Erro) {
	janela := uintptr(p.JanelaMae)
	// A chave tem de ser a do certificado (COMPARE_KEY): chave trocada no dispositivo produziria uma
	// assinatura que ninguém confere (o host também a confere contra o certificado, depois).
	flags := uint32(win.CRYPT_ACQUIRE_COMPARE_KEY_FLAG)
	if preferirCNG {
		flags |= win.CRYPT_ACQUIRE_PREFER_NCRYPT_KEY_FLAG
	}
	var parametros unsafe.Pointer
	if janela != 0 {
		// Para o CSP, a janela vai antes de adquirir a chave; o CRYPT_ACQUIRE_WINDOW_HANDLE_FLAG a
		// entrega também ao provedor que abre diálogo já na aquisição. Falhar aqui não impede a
		// assinatura: o diálogo só abre atrás da janela do navegador (medição (e) da F0).
		_ = definirJanelaDoCSP(janela)
		flags |= win.CRYPT_ACQUIRE_WINDOW_HANDLE_FLAG
		parametros = unsafe.Pointer(&janela)
	}
	var chave win.Handle
	var keySpec uint32
	var liberar bool
	if err := win.CryptAcquireCertificatePrivateKey(ctx, flags, parametros, &chave, &keySpec, &liberar); err != nil {
		return nil, "", erroDoWindows("abrir a chave", codigoDoErro(err))
	}
	runtime.KeepAlive(janela)
	if keySpec == win.CERT_NCRYPT_KEY_SPEC {
		if liberar {
			defer liberarChaveCNG(chave)
		}
		if janela != 0 {
			_ = definirJanelaDaChaveCNG(chave, janela)
		}
		assinada, codigo := assinarCNG(chave, digest)
		if codigo != 0 {
			return nil, CaminhoCNG, erroDoWindows("assinar", codigo)
		}
		return assinada, CaminhoCNG, nil
	}
	if liberar {
		defer win.CryptReleaseContext(chave, 0)
	}
	assinada, etapa, codigo := assinarCSP(chave, keySpec, digest)
	if codigo != 0 {
		return nil, CaminhoCSP, erroDoWindows(etapa, codigo)
	}
	return assinada, CaminhoCSP, nil
}

// Diagnosticar diz, por provedor de chave, quantos certificados com chave o repositório tem. Sem
// nenhum, o próprio repositório aparece carregado (o diagnóstico não confunde "sem certificado" com
// "sem programa de cartão").
func (p *Provedor) Diagnosticar(_ context.Context) []assinatura.RelatorioDoProvedor {
	vs, err := p.vistos()
	if err != nil {
		return []assinatura.RelatorioDoProvedor{{Nome: NomeDoRepositorio, Estado: assinatura.EstadoFalhou, Detalhe: erroDoWindows("abrir o repositório", codigoDoErro(err)).Detalhe}}
	}
	if len(vs) == 0 {
		return []assinatura.RelatorioDoProvedor{{Nome: NomeDoRepositorio, Estado: assinatura.EstadoCarregado, Detalhe: "nenhum certificado com chave privada"}}
	}
	porProvedor := map[string]*assinatura.RelatorioDoProvedor{}
	for _, v := range vs {
		nome := v.provedor
		if nome == "" {
			nome = RotuloSemNome
		}
		r, ok := porProvedor[nome]
		if !ok {
			r = &assinatura.RelatorioDoProvedor{Nome: nome, Estado: assinatura.EstadoCarregado, Detalhe: caminhoDoTipo(v.tipo)}
			porProvedor[nome] = r
		}
		r.Vistos = append(r.Vistos, certificadoDe(v))
		r.Certificados = len(r.Vistos)
	}
	nomes := make([]string, 0, len(porProvedor))
	for nome := range porProvedor {
		nomes = append(nomes, nome)
	}
	sort.Strings(nomes)
	saida := make([]assinatura.RelatorioDoProvedor, 0, len(nomes))
	for _, nome := range nomes {
		saida = append(saida, *porProvedor[nome])
	}
	return saida
}
