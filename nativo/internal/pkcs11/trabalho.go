//go:build !windows

package pkcs11

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	p11 "github.com/miekg/pkcs11"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Tetos contra módulo que devolve lista sem fim.
const (
	maximoDeSlots   = 32
	maximoDeObjetos = 64
)

// infoDoModulo é o que o módulo diz de si (C_GetInfo), para o diagnóstico.
type infoDoModulo struct {
	Fabricante string `json:"fabricante"`
	Descricao  string `json:"descricao"`
	Cryptoki   string `json:"cryptoki"`
	Biblioteca string `json:"biblioteca"`
}

// certificadoDoModulo é um certificado que o filho achou num token.
type certificadoDoModulo struct {
	Ref             string `json:"ref"`
	DER             []byte `json:"der"`
	Leitor          string `json:"leitor,omitempty"`
	ExigePin        bool   `json:"exigePin"`
	EstadoDoPin     string `json:"estadoDoPin"`
	ChaveConfirmada bool   `json:"chaveConfirmada"`
}

// sessaoDeTrabalho é o módulo aberto neste processo (o filho). As sessões abertas e os logins
// feitos são anotados para `fechar`, que o filho só chama DEPOIS de responder: C_Logout,
// C_CloseSession e C_Finalize de fabricante podem travar ou derrubar o processo, e isso não pode
// custar uma resposta que já existe (o pai espera `esperaDoFim` e mata).
type sessaoDeTrabalho struct {
	caminho string
	ctx     *p11.Ctx
	abertas []p11.SessionHandle
	logadas []p11.SessionHandle
}

func abrirModulo(caminho string) (*sessaoDeTrabalho, *protocolo.Erro) {
	ctx := p11.New(caminho)
	if ctx == nil {
		return nil, protocolo.Novo(protocolo.ModuloFalhou, "o módulo não carregou")
	}
	if err := ctx.Initialize(); err != nil {
		ctx.Destroy()
		return nil, protocolo.Novo(protocolo.ModuloFalhou, "C_Initialize: "+nomeDoErro(err))
	}
	return &sessaoDeTrabalho{caminho: caminho, ctx: ctx}, nil
}

// fechar encerra o que o trabalho abriu: o logout só do login que ESTE processo fez (o token que já
// estava logado por outra aplicação continua como estava), as sessões, e o módulo.
func (s *sessaoDeTrabalho) fechar() {
	for _, sessao := range s.logadas {
		_ = s.ctx.Logout(sessao)
	}
	for _, sessao := range s.abertas {
		_ = s.ctx.CloseSession(sessao)
	}
	_ = s.ctx.Finalize()
	s.ctx.Destroy()
}

// abrirSessao abre uma sessão só de leitura (o login de usuário não exige escrita) e a anota.
func (s *sessaoDeTrabalho) abrirSessao(slot uint) (p11.SessionHandle, error) {
	sessao, err := s.ctx.OpenSession(slot, p11.CKF_SERIAL_SESSION)
	if err == nil {
		s.abertas = append(s.abertas, sessao)
	}
	return sessao, err
}

func (s *sessaoDeTrabalho) info() infoDoModulo {
	i, err := s.ctx.GetInfo()
	if err != nil {
		return infoDoModulo{}
	}
	return infoDoModulo{
		Fabricante: strings.TrimSpace(i.ManufacturerID),
		Descricao:  strings.TrimSpace(i.LibraryDescription),
		Cryptoki:   versao(i.CryptokiVersion),
		Biblioteca: versao(i.LibraryVersion),
	}
}

func versao(v p11.Version) string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// ulong lê um CK_ULONG de atributo, na ordem nativa (8 bytes fora do Windows de 64 bits, 4 nos de
// 32).
func ulong(b []byte) (uint64, bool) {
	switch len(b) {
	case 8:
		return binary.NativeEndian.Uint64(b), true
	case 4:
		return uint64(binary.NativeEndian.Uint32(b)), true
	}
	return 0, false
}

// objetos devolve os handles da classe pedida (com teto), terminando a busca em qualquer caso.
func (s *sessaoDeTrabalho) objetos(sessao p11.SessionHandle, modelo []*p11.Attribute) []p11.ObjectHandle {
	if err := s.ctx.FindObjectsInit(sessao, modelo); err != nil {
		return nil
	}
	defer func() { _ = s.ctx.FindObjectsFinal(sessao) }()
	var todos []p11.ObjectHandle
	for len(todos) < maximoDeObjetos {
		lote, _, err := s.ctx.FindObjects(sessao, maximoDeObjetos-len(todos))
		if err != nil || len(lote) == 0 {
			break
		}
		todos = append(todos, lote...)
	}
	return todos
}

// certificadoNoSlot é um certificado X.509 lido de um token, com o CKA_ID.
type certificadoNoSlot struct {
	der  []byte
	id   []byte
	cert *x509.Certificate
}

func (s *sessaoDeTrabalho) certificados(sessao p11.SessionHandle) []certificadoNoSlot {
	var saida []certificadoNoSlot
	for _, o := range s.objetos(sessao, []*p11.Attribute{
		p11.NewAttribute(p11.CKA_CLASS, p11.CKO_CERTIFICATE),
		p11.NewAttribute(p11.CKA_CERTIFICATE_TYPE, p11.CKC_X_509),
	}) {
		attrs, err := s.ctx.GetAttributeValue(sessao, o, []*p11.Attribute{
			p11.NewAttribute(p11.CKA_VALUE, nil),
			p11.NewAttribute(p11.CKA_ID, nil),
		})
		if err != nil || len(attrs) != 2 || len(attrs[0].Value) == 0 {
			continue
		}
		c := certificadoNoSlot{der: attrs[0].Value, id: attrs[1].Value}
		c.cert, _ = x509.ParseCertificate(c.der)
		saida = append(saida, c)
	}
	return saida
}

// chavePrivada é uma chave privada visível na sessão, com o que serve para casá-la com um
// certificado: o CKA_ID e, se o token deixar ler, o módulo RSA.
type chavePrivada struct {
	handle p11.ObjectHandle
	id     []byte
	modulo []byte
}

func (s *sessaoDeTrabalho) chavesPrivadas(sessao p11.SessionHandle) []chavePrivada {
	var saida []chavePrivada
	for _, o := range s.objetos(sessao, []*p11.Attribute{p11.NewAttribute(p11.CKA_CLASS, p11.CKO_PRIVATE_KEY)}) {
		k := chavePrivada{handle: o}
		if attrs, err := s.ctx.GetAttributeValue(sessao, o, []*p11.Attribute{p11.NewAttribute(p11.CKA_ID, nil)}); err == nil && len(attrs) == 1 {
			k.id = attrs[0].Value
		}
		if attrs, err := s.ctx.GetAttributeValue(sessao, o, []*p11.Attribute{p11.NewAttribute(p11.CKA_MODULUS, nil)}); err == nil && len(attrs) == 1 {
			k.modulo = attrs[0].Value
		}
		saida = append(saida, k)
	}
	return saida
}

// casar acha a chave do certificado: pelo CKA_ID (o comum) ou, quando o ID falta ou não casa, pelo
// módulo RSA da chave igual ao do certificado. O CKA_ID só vale se a chave não DESMENTIR o
// certificado: a chave com o mesmo ID cujo módulo se lê e é outro (cartão renovado que manteve o ID
// da chave antiga) é pulada, e a busca segue pelo módulo.
func casar(c certificadoNoSlot, chaves []chavePrivada) (chavePrivada, bool) {
	var moduloDoCertificado []byte
	if c.cert != nil {
		if pub, ok := c.cert.PublicKey.(*rsa.PublicKey); ok {
			moduloDoCertificado = pub.N.Bytes()
		}
	}
	mesmoModulo := func(k chavePrivada) bool {
		return moduloDoCertificado != nil && len(k.modulo) > 0 && bytes.Equal(bytes.TrimLeft(k.modulo, "\x00"), moduloDoCertificado)
	}
	desmente := func(k chavePrivada) bool {
		return moduloDoCertificado != nil && len(k.modulo) > 0 && !mesmoModulo(k)
	}
	if len(c.id) > 0 {
		for _, k := range chaves {
			if bytes.Equal(k.id, c.id) && !desmente(k) {
				return k, true
			}
		}
	}
	for _, k := range chaves {
		if mesmoModulo(k) {
			return k, true
		}
	}
	return chavePrivada{}, false
}

// listar percorre os slots com token e devolve os certificados que têm (ou podem ter) chave: com a
// chave VISTA antes do login (`ChaveConfirmada`), ou, se o token exige login, também os outros,
// porque a chave pode estar escondida até o PIN (`CKA_PRIVATE`), e há token que mostra umas chaves
// e esconde outras. Esconder o certificado de uma chave escondida impediria a pessoa de assinar;
// listar um certificado sem chave custa um `chave-ausente` claro depois do login. A AC e o
// certificado sem uso de assinatura, quem tira é o host (`assinatura.Listavel`). Nunca faz login.
func (s *sessaoDeTrabalho) listar() ([]certificadoDoModulo, *protocolo.Erro) {
	slots, err := s.ctx.GetSlotList(true)
	if err != nil {
		return nil, protocolo.Novo(protocolo.ModuloFalhou, "C_GetSlotList: "+nomeDoErro(err))
	}
	if len(slots) > maximoDeSlots {
		slots = slots[:maximoDeSlots]
	}
	var saida []certificadoDoModulo
	for _, slot := range slots {
		leitor := ""
		if si, err := s.ctx.GetSlotInfo(slot); err == nil {
			leitor = strings.TrimSpace(si.SlotDescription)
		}
		ti, err := s.ctx.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		sessao, err := s.abrirSessao(slot)
		if err != nil {
			continue
		}
		chaves := s.chavesPrivadas(sessao)
		podeEsconder := ti.Flags&p11.CKF_LOGIN_REQUIRED != 0
		for _, c := range s.certificados(sessao) {
			_, confirmada := casar(c, chaves)
			if !confirmada && !podeEsconder {
				continue
			}
			saida = append(saida, certificadoDoModulo{
				Ref:             assinatura.Ref(c.der),
				DER:             c.der,
				Leitor:          leitor,
				ExigePin:        ti.Flags&p11.CKF_LOGIN_REQUIRED != 0 && ti.Flags&p11.CKF_PROTECTED_AUTHENTICATION_PATH == 0,
				EstadoDoPin:     estadoDoPin(ti.Flags),
				ChaveConfirmada: confirmada,
			})
		}
	}
	return saida, nil
}

// acharPorRef acha o slot e o certificado de `ref`, numa sessão aberta nele (as sessões são
// fechadas por `fechar`, depois da resposta).
func (s *sessaoDeTrabalho) acharPorRef(ref string) (uint, p11.SessionHandle, certificadoNoSlot, bool) {
	slots, err := s.ctx.GetSlotList(true)
	if err != nil {
		return 0, 0, certificadoNoSlot{}, false
	}
	for i, slot := range slots {
		if i >= maximoDeSlots {
			break
		}
		sessao, err := s.abrirSessao(slot)
		if err != nil {
			continue
		}
		for _, c := range s.certificados(sessao) {
			h := sha256.Sum256(c.der)
			if hex.EncodeToString(h[:]) == ref {
				return slot, sessao, c, true
			}
		}
	}
	return 0, 0, certificadoNoSlot{}, false
}

// entrar faz um C_Login e traduz a falha, relendo as flags do token depois dela. O login de usuário
// que ESTE processo fez é anotado para o logout em `fechar`; `CKR_USER_ALREADY_LOGGED_IN` (há
// middleware que compartilha o login entre aplicações) segue sem logout nosso no fim.
func (s *sessaoDeTrabalho) entrar(slot uint, sessao p11.SessionHandle, usuario uint, pin []byte) *protocolo.Erro {
	_, err := entrarNoToken(s.caminho, sessao, usuario, pin)
	if err == nil {
		if usuario == usuarioComum {
			s.logadas = append(s.logadas, sessao)
		}
		return nil
	}
	if e, ok := codigoCkr(err); ok && e == p11.CKR_USER_ALREADY_LOGGED_IN {
		return nil
	}
	var flags uint
	if ti, errTi := s.ctx.GetTokenInfo(slot); errTi == nil {
		flags = ti.Flags
	}
	return erroDoLogin(err, flags)
}

// assinar entra no token do certificado `ref` e assina o DigestInfo SHA-256 de `digest` com
// CKM_RSA_PKCS. Com CKF_PROTECTED_AUTHENTICATION_PATH o login vai sem PIN (o leitor ou o middleware
// pedem); sem ele, PIN vazio nunca vai ao cartão (há middleware que conta como tentativa errada).
// Nunca repete login com PIN errado. O PIN é zerado ao sair, em todo caminho.
func (s *sessaoDeTrabalho) assinar(ref string, digest [32]byte, pin []byte) ([]byte, *protocolo.Erro) {
	defer assinatura.Zerar(pin)
	slot, sessao, cert, ok := s.acharPorRef(ref)
	if !ok {
		return nil, protocolo.Novo(protocolo.CertificadoNaoEncontrado, "nenhum token presente tem o certificado")
	}
	ti, err := s.ctx.GetTokenInfo(slot)
	if err != nil {
		return nil, protocolo.Novo(protocolo.ModuloFalhou, "C_GetTokenInfo: "+nomeDoErro(err))
	}
	if ti.Flags&p11.CKF_USER_PIN_LOCKED != 0 {
		return nil, protocolo.Novo(protocolo.TokenBloqueado, "o token diz que o PIN está bloqueado")
	}
	protegido := ti.Flags&p11.CKF_PROTECTED_AUTHENTICATION_PATH != 0
	exigeLogin := ti.Flags&p11.CKF_LOGIN_REQUIRED != 0
	pinDoLogin := pin
	if protegido {
		pinDoLogin = nil
	} else if exigeLogin && len(pin) == 0 {
		return nil, protocolo.Novo(protocolo.Protocolo, "o token exige PIN e ele não veio")
	}
	if exigeLogin {
		if e := s.entrar(slot, sessao, usuarioComum, pinDoLogin); e != nil {
			return nil, e
		}
	}

	chave, ok := casar(cert, s.chavesPrivadas(sessao))
	if !ok {
		return nil, protocolo.Novo(protocolo.ChaveAusente, "o token não tem a chave privada do certificado")
	}
	if attrs, err := s.ctx.GetAttributeValue(sessao, chave.handle, []*p11.Attribute{p11.NewAttribute(p11.CKA_KEY_TYPE, nil)}); err == nil && len(attrs) == 1 {
		if tipo, ok := ulong(attrs[0].Value); ok && tipo != p11.CKK_RSA {
			return nil, protocolo.Novo(protocolo.AlgoritmoNaoSuportado, "a chave não é RSA")
		}
	}
	sempreAutentica := false
	if attrs, err := s.ctx.GetAttributeValue(sessao, chave.handle, []*p11.Attribute{p11.NewAttribute(p11.CKA_ALWAYS_AUTHENTICATE, nil)}); err == nil && len(attrs) == 1 && len(attrs[0].Value) == 1 {
		sempreAutentica = attrs[0].Value[0] != 0
	}

	bloco := assinatura.DigestInfo(digest)
	// assinarUmaVez devolve também se o C_Sign recusou por falta do login da operação.
	assinarUmaVez := func(comLoginDaOperacao bool) ([]byte, *protocolo.Erro, bool) {
		if err := s.ctx.SignInit(sessao, []*p11.Mechanism{p11.NewMechanism(p11.CKM_RSA_PKCS, nil)}, chave.handle); err != nil {
			return nil, erroDaAssinatura("C_SignInit", err), false
		}
		if comLoginDaOperacao {
			// Chave com CKA_ALWAYS_AUTHENTICATE pede o PIN de novo para CADA operação.
			if e := s.entrar(slot, sessao, usuarioDaOperacao, pinDoLogin); e != nil {
				return nil, e, false
			}
		}
		saida, err := s.ctx.Sign(sessao, bloco)
		if err != nil {
			c, ok := codigoCkr(err)
			return nil, erroDaAssinatura("C_Sign", err), ok && c == p11.CKR_USER_NOT_LOGGED_IN
		}
		return saida, nil, false
	}
	saida, e, faltouLogin := assinarUmaVez(sempreAutentica && exigeLogin)
	if e != nil && faltouLogin && !sempreAutentica && exigeLogin {
		// Token que exige o login da operação sem declarar CKA_ALWAYS_AUTHENTICATE: uma vez, com o
		// mesmo PIN (não é repetir PIN errado: o primeiro login deu certo).
		saida, e, _ = assinarUmaVez(true)
	}
	if e != nil {
		return nil, e
	}
	return saida, nil
}
