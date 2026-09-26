// Package bilhete confere o BILHETE: o JWS ES256 com que o servidor que preparou o resumo autoriza
// o programa a assiná-lo. Sem bilhete conferido, o programa não assina (regra 1 do CLAUDE.md).
//
// A referência é `conferirBilhete` de `@confidata/icp-brasil` (`src/bilhete.ts`), e o contrato
// entre as duas implementações são as fixtures de `protocolo/fixtures/bilhete/`: cada caso diz o
// código E a etapa em que a conferência para, e `bilhete_test.go` roda todos. As regras que o Go
// precisa cumprir para concordar com o TypeScript estão no `LEIAME.md` das fixtures, cada uma com o
// caso que a prova.
//
// A ordem, parando na primeira falha: forma, alg, typ, kid, assinatura, carga (objeto), v, carga
// (campos), iss, aud, padrao, dig, cer, tempo.
package bilhete

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Constantes do bilhete, iguais às de `protocolo.json` das fixtures (conferidas em teste).
const (
	Tipo                  = "assinador+jws"
	Algoritmo             = "ES256"
	Versao                = 1
	ValidadeS             = 300
	ToleranciaS           = 900
	TamanhoMaximo         = protocolo.TamanhoMaximoDoBilhete
	LimiteDoDocumento     = 200
	LimiteDaOrganizacao   = 120
	InteiroMaximo         = 1<<53 - 1
	FinalidadeAssinatura  = "assinatura"
	FinalidadeVerificacao = "verificacao"
)

// FaixasDeControle são os pontos de código que saem de `doc` e `org` antes de a janela os mostrar:
// controle (C0, DEL, C1), os que reordenam o texto (marca árabe, LRM, RLM, embutimentos e
// isolamentos bidirecionais), os separadores de linha e parágrafo, e o BOM.
var FaixasDeControle = [][2]rune{
	{0x0000, 0x001f},
	{0x007f, 0x009f},
	{0x061c, 0x061c},
	{0x200e, 0x200f},
	{0x2028, 0x202e},
	{0x2066, 0x2069},
	{0xfeff, 0xfeff},
}

// FaixasDeEspaco são os espaços que, sem os controles, fazem um texto "em branco".
var FaixasDeEspaco = [][2]rune{
	{0x0020, 0x0020},
	{0x00a0, 0x00a0},
	{0x1680, 0x1680},
	{0x2000, 0x200a},
	{0x202f, 0x202f},
	{0x205f, 0x205f},
	{0x3000, 0x3000},
}

// Formatos de campo da carga (os `formatosDaCarga` das fixtures).
var (
	FormatoDeAud = `^[\x21-\x7e]{1,256}$`
	FormatoDeSid = `^[A-Za-z0-9_-]{1,64}$`
	FormatoHex   = `^[0-9a-f]{64}$`

	regexAud = regexp.MustCompile(FormatoDeAud)
	regexSid = regexp.MustCompile(FormatoDeSid)
	regexHex = regexp.MustCompile(FormatoHex)
)

var alfabetoBase64Url = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

// Etapa é onde a conferência parou.
type Etapa string

const (
	EtapaForma      Etapa = "forma"
	EtapaAlg        Etapa = "alg"
	EtapaTyp        Etapa = "typ"
	EtapaKid        Etapa = "kid"
	EtapaAssinatura Etapa = "assinatura"
	EtapaCarga      Etapa = "carga"
	EtapaV          Etapa = "v"
	EtapaIss        Etapa = "iss"
	EtapaAud        Etapa = "aud"
	EtapaPadrao     Etapa = "padrao"
	EtapaDig        Etapa = "dig"
	EtapaCer        Etapa = "cer"
	EtapaTempo      Etapa = "tempo"
)

// Ordem é a sequência das etapas (a carga aparece duas vezes: objeto, depois campos).
var Ordem = []Etapa{EtapaForma, EtapaAlg, EtapaTyp, EtapaKid, EtapaAssinatura, EtapaCarga, EtapaV, EtapaCarga, EtapaIss, EtapaAud, EtapaPadrao, EtapaDig, EtapaCer, EtapaTempo}

// CodigoDaEtapa é o código de erro de cada etapa.
var CodigoDaEtapa = map[Etapa]protocolo.Codigo{
	EtapaForma:      protocolo.BilheteInvalido,
	EtapaAlg:        protocolo.BilheteInvalido,
	EtapaTyp:        protocolo.BilheteInvalido,
	EtapaKid:        protocolo.BilheteInvalido,
	EtapaAssinatura: protocolo.BilheteInvalido,
	EtapaCarga:      protocolo.BilheteInvalido,
	EtapaV:          protocolo.BilheteInvalido,
	EtapaIss:        protocolo.BilheteInvalido,
	EtapaAud:        protocolo.OrigemRecusada,
	EtapaPadrao:     protocolo.OrigemRecusada,
	EtapaDig:        protocolo.DigestDivergente,
	EtapaCer:        protocolo.CertificadoDivergente,
	EtapaTempo:      protocolo.Relogio,
}

// Chave é uma chave PÚBLICA pinada de emissor de bilhete.
type Chave struct {
	Kid      string
	Emissor  string
	Ambiente string
	publica  *ecdsa.PublicKey
}

// ChaveDeJwk monta uma chave pinada a partir das partes da JWK P-256, conferindo o formato (x e y
// em base64url canônico de 32 bytes, ponto na curva), o emissor e o ambiente.
func ChaveDeJwk(kid, emissor, ambiente, x, y string) (Chave, error) {
	if kid == "" {
		return Chave{}, errors.New("kid vazio")
	}
	if _, ok := origem.Padroes[emissor]; !ok {
		return Chave{}, errors.New("emissor desconhecido")
	}
	if ambiente != origem.AmbienteProducao && ambiente != origem.AmbienteDev && ambiente != origem.AmbienteTeste {
		return Chave{}, errors.New("ambiente desconhecido")
	}
	bx, errX := base64Estrito(x)
	by, errY := base64Estrito(y)
	if errX != nil || errY != nil || len(bx) != 32 || len(by) != 32 || !alfabetoBase64Url.MatchString(x) || !alfabetoBase64Url.MatchString(y) {
		return Chave{}, errors.New("x e y da JWK precisam ser base64url de 32 bytes")
	}
	ponto := append(append([]byte{0x04}, bx...), by...)
	publica, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), ponto)
	if err != nil {
		return Chave{}, errors.New("ponto fora da curva P-256")
	}
	return Chave{Kid: kid, Emissor: emissor, Ambiente: ambiente, publica: publica}, nil
}

// Bilhete é a carga conferida. `Doc` e `Org` já saem sem os pontos de controle.
type Bilhete struct {
	V   int    `json:"v"`
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sid string `json:"sid"`
	Dig string `json:"dig"`
	Cer string `json:"cer"`
	Fin string `json:"fin"`
	Doc string `json:"doc"`
	Org string `json:"org"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// Entrada é o que o programa sabe do pedido, fora do bilhete.
type Entrada struct {
	// Origem da página, como a extensão a informou.
	Origem string
	// Digest pedido em `assinar`, em hexadecimal minúsculo.
	Digest string
	// CertificadoSha256 é o SHA-256 do DER do certificado de `ref`, em hexadecimal minúsculo.
	CertificadoSha256 string
	Agora             time.Time
}

// Resultado da conferência. Com `OK`, `Bilhete` e `Chave`; sem, `Codigo` e `Etapa`.
type Resultado struct {
	OK      bool
	Bilhete Bilhete
	Chave   Chave
	Codigo  protocolo.Codigo
	Etapa   Etapa
}

// Erro devolve a recusa como erro do protocolo, com a etapa no detalhe (para o suporte).
func (r Resultado) Erro() *protocolo.Erro {
	if r.OK {
		return nil
	}
	return protocolo.Novo(r.Codigo, "etapa "+string(r.Etapa))
}

func recusa(e Etapa) Resultado {
	return Resultado{Codigo: CodigoDaEtapa[e], Etapa: e}
}

var camposDoCabecalho = map[string]bool{"alg": true, "typ": true, "kid": true}

var camposDaCarga = []string{"v", "iss", "aud", "sid", "dig", "cer", "fin", "doc", "org", "iat", "exp"}

// base64Estrito decodifica base64url SEM preenchimento e só na forma CANÔNICA. Quem chama já
// conferiu o alfabeto: o `Strict()` do Go pula `\r` e `\n` em silêncio.
func base64Estrito(s string) ([]byte, error) {
	return base64.RawURLEncoding.Strict().DecodeString(s)
}

// objetoJSON decodifica um objeto JSON com o decodificador padrão (números em float64), como o
// contrato pede: UTF-8 inválido é recusado ANTES (o `encoding/json` o trocaria por U+FFFD em
// silêncio), e o que não é objeto é recusado.
func objetoJSON(b []byte) (map[string]any, bool) {
	if !utf8.Valid(b) {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

func inteiroNaoNegativo(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || f < 0 || f > InteiroMaximo {
		return 0, false
	}
	return int64(f), true
}

func naFaixa(r rune, faixas [][2]rune) bool {
	for _, f := range faixas {
		if r >= f[0] && r <= f[1] {
			return true
		}
	}
	return false
}

// semControle tira os pontos de `FaixasDeControle`. Surrogate solto já chegou como U+FFFD: é o que
// o `encoding/json` faz com `\ud800`, e é o que a referência faz na saída.
func semControle(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !naFaixa(r, FaixasDeControle) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// emBranco: sem os controles, só sobra espaço de `FaixasDeEspaco` (ou nada).
func emBranco(s string) bool {
	for _, r := range semControle(s) {
		if !naFaixa(r, FaixasDeEspaco) {
			return false
		}
	}
	return true
}

func texto(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

// Conferir confere o bilhete contra as chaves pinadas e o pedido, na ordem do contrato, e para na
// primeira falha. Nunca entra em pânico com entrada hostil.
func Conferir(jws string, chaves []Chave, e Entrada) (resultado Resultado) {
	defer func() {
		if recover() != nil {
			resultado = recusa(EtapaForma)
		}
	}()

	// forma
	if len(jws) == 0 || len(jws) > TamanhoMaximo {
		return recusa(EtapaForma)
	}
	partes := strings.Split(jws, ".")
	if len(partes) != 3 {
		return recusa(EtapaForma)
	}
	for _, p := range partes {
		if !alfabetoBase64Url.MatchString(p) {
			return recusa(EtapaForma)
		}
	}
	bytesDoCabecalho, err := base64Estrito(partes[0])
	if err != nil {
		return recusa(EtapaForma)
	}
	cabecalho, ok := objetoJSON(bytesDoCabecalho)
	if !ok {
		return recusa(EtapaForma)
	}
	for k := range cabecalho {
		if !camposDoCabecalho[k] {
			return recusa(EtapaForma)
		}
	}

	// alg, typ
	if alg, _ := texto(cabecalho["alg"]); alg != Algoritmo {
		return recusa(EtapaAlg)
	}
	if typ, _ := texto(cabecalho["typ"]); typ != Tipo {
		return recusa(EtapaTyp)
	}

	// kid: igual, byte a byte, a exatamente uma chave pinada
	kid, ok := texto(cabecalho["kid"])
	if !ok {
		return recusa(EtapaKid)
	}
	var chave Chave
	candidatas := 0
	for _, c := range chaves {
		if c.publica != nil && c.Kid == kid {
			chave = c
			candidatas++
		}
	}
	if candidatas != 1 {
		return recusa(EtapaKid)
	}

	// assinatura
	assinatura, err := base64Estrito(partes[2])
	if err != nil || len(assinatura) != 64 {
		return recusa(EtapaAssinatura)
	}
	resumo := sha256.Sum256([]byte(partes[0] + "." + partes[1]))
	r := new(big.Int).SetBytes(assinatura[:32])
	s := new(big.Int).SetBytes(assinatura[32:])
	if !ecdsa.Verify(chave.publica, resumo[:], r, s) {
		return recusa(EtapaAssinatura)
	}

	// carga: objeto
	bytesDaCarga, err := base64Estrito(partes[1])
	if err != nil {
		return recusa(EtapaCarga)
	}
	carga, ok := objetoJSON(bytesDaCarga)
	if !ok {
		return recusa(EtapaCarga)
	}

	// v
	if v, ok := carga["v"].(float64); !ok || v != Versao {
		return recusa(EtapaV)
	}

	// carga: campos
	if len(carga) != len(camposDaCarga) {
		return recusa(EtapaCarga)
	}
	for _, k := range camposDaCarga {
		if _, ok := carga[k]; !ok {
			return recusa(EtapaCarga)
		}
	}
	iss, ok1 := texto(carga["iss"])
	aud, ok2 := texto(carga["aud"])
	sid, ok3 := texto(carga["sid"])
	dig, ok4 := texto(carga["dig"])
	cer, ok5 := texto(carga["cer"])
	fin, ok6 := texto(carga["fin"])
	doc, ok7 := texto(carga["doc"])
	org, ok8 := texto(carga["org"])
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7 && ok8) {
		return recusa(EtapaCarga)
	}
	if !regexAud.MatchString(aud) || !regexSid.MatchString(sid) || !regexHex.MatchString(dig) || !regexHex.MatchString(cer) {
		return recusa(EtapaCarga)
	}
	if fin != FinalidadeAssinatura && fin != FinalidadeVerificacao {
		return recusa(EtapaCarga)
	}
	pontosDoDoc, pontosDaOrg := utf8.RuneCountInString(doc), utf8.RuneCountInString(org)
	if pontosDoDoc < 1 || pontosDoDoc > LimiteDoDocumento || pontosDaOrg < 1 || pontosDaOrg > LimiteDaOrganizacao {
		return recusa(EtapaCarga)
	}
	if emBranco(doc) || emBranco(org) {
		return recusa(EtapaCarga)
	}
	iat, okIat := inteiroNaoNegativo(carga["iat"])
	exp, okExp := inteiroNaoNegativo(carga["exp"])
	if !okIat || !okExp || exp-iat != ValidadeS {
		return recusa(EtapaCarga)
	}

	// iss
	if iss != chave.Emissor {
		return recusa(EtapaIss)
	}

	// aud, padrao
	if aud != e.Origem {
		return recusa(EtapaAud)
	}
	if !origem.Aceita(aud, chave.Emissor, chave.Ambiente) {
		return recusa(EtapaPadrao)
	}

	// dig, cer
	if dig != e.Digest {
		return recusa(EtapaDig)
	}
	if cer != e.CertificadoSha256 {
		return recusa(EtapaCer)
	}

	// tempo, com as duas bordas inclusivas
	agora := e.Agora.Unix()
	if agora < iat-ToleranciaS || agora > exp+ToleranciaS {
		return recusa(EtapaTempo)
	}

	return Resultado{
		OK: true,
		Bilhete: Bilhete{
			V: Versao, Iss: chave.Emissor, Aud: aud, Sid: sid, Dig: dig, Cer: cer, Fin: fin,
			Doc: semControle(doc), Org: semControle(org), Iat: iat, Exp: exp,
		},
		Chave: chave,
	}
}
