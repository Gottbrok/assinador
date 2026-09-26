package protocolo

// Versao é o `v` de toda mensagem entre a extensão e o programa.
const Versao = 1

// Limites do quadro de native messaging. A entrada é pequena (o maior pedido é um `assinar`, com
// bilhete de até 4 KiB), e o teto corta abuso; a saída é o limite do Chrome para o que o programa
// envia.
const (
	EntradaMaxima = 64 * 1024
	SaidaMaxima   = 1024 * 1024
)

// TamanhoMaximoDoBilhete é o teto do JWS, igual ao `TAMANHO_MAXIMO_DO_BILHETE` da biblioteca.
const TamanhoMaximoDoBilhete = 4 * 1024

// TetoDoPin cobre folgado os PINs de cartão e token (em geral de 4 a 16 dígitos), em bytes.
const TetoDoPin = 64

// Operações da extensão para o programa. `conferir` é só desta ponte: a página não a vê, e a
// janela de confirmação da extensão mostra o que ela devolve.
const (
	OpOla         = "ola"
	OpListar      = "listar"
	OpConferir    = "conferir"
	OpAssinar     = "assinar"
	OpDiagnostico = "diagnostico"
)

// Estados do PIN na lista, das flags `CKF_USER_PIN_*` do token.
const (
	PinOk               = "ok"
	PinPoucasTentativas = "poucas-tentativas"
	PinUltimaTentativa  = "ultima-tentativa"
	PinBloqueado        = "bloqueado"
)

// Resposta é o envelope de toda resposta do programa. Com `ok`, vai `dados`; sem, vai `erro`.
type Resposta struct {
	V     int             `json:"v"`
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Dados any             `json:"dados,omitempty"`
	Erro  *ErroNaResposta `json:"erro,omitempty"`
}

// ErroNaResposta é o `erro` do envelope.
type ErroNaResposta struct {
	Codigo  Codigo `json:"codigo"`
	Detalhe any    `json:"detalhe,omitempty"`
}

// Sucesso monta a resposta de uma operação que deu certo.
func Sucesso(id string, dados any) Resposta {
	return Resposta{V: Versao, ID: id, OK: true, Dados: dados}
}

// Falha monta a resposta de uma recusa.
func Falha(id string, e *Erro) Resposta {
	return Resposta{V: Versao, ID: id, OK: false, Erro: &ErroNaResposta{Codigo: e.Codigo, Detalhe: e.DetalheParaResposta()}}
}

// DadosDoOla é o que o programa diz de si. A extensão acrescenta a versão dela.
type DadosDoOla struct {
	Versao     string `json:"versao"`
	Protocolo  int    `json:"protocolo"`
	Plataforma string `json:"plataforma"`
}

// CertificadoListado é um item de `listar`, na forma que a biblioteca espera
// (`CertificadoListado` de `assinadorProtocolo.ts`).
type CertificadoListado struct {
	Ref              string `json:"ref"`
	DER              string `json:"der"`
	Provedor         string `json:"provedor"`
	RotuloDoProvedor string `json:"rotuloDoProvedor"`
	Leitor           string `json:"leitor,omitempty"`
	ExigePin         bool   `json:"exigePin"`
	EstadoDoPin      string `json:"estadoDoPin,omitempty"`
}

// DadosDoListar é a resposta de `listar`.
type DadosDoListar struct {
	Certificados []CertificadoListado `json:"certificados"`
	Avisos       []string             `json:"avisos"`
}

// DadosDoConferir é o que a janela de confirmação mostra. O assunto sai com os dígitos
// mascarados: o CN ICP-Brasil é `NOME:CPF`, e o programa não interpreta campo ICP-Brasil (quem lê
// nome e documento é a biblioteca, no navegador).
type DadosDoConferir struct {
	Emissor     string                  `json:"emissor"`
	Organizacao string                  `json:"organizacao"`
	Documento   string                  `json:"documento"`
	Finalidade  string                  `json:"finalidade"`
	ExpiraEm    string                  `json:"expiraEm"`
	Certificado CertificadoParaConferir `json:"certificado"`
}

// CertificadoParaConferir é o certificado escolhido, como a janela o mostra.
type CertificadoParaConferir struct {
	Assunto   string `json:"assunto"`
	Emissor   string `json:"emissor"`
	ValidoAte string `json:"validoAte"`
}

// DadosDoAssinar leva a assinatura RSA PKCS#1 v1.5 com DigestInfo SHA-256, em base64.
type DadosDoAssinar struct {
	Assinatura string `json:"assinatura"`
}

// DadosDoDiagnostico é o relatório para o suporte (sem CPF) e o mesmo relatório em frases.
type DadosDoDiagnostico struct {
	Relatorio any    `json:"relatorio"`
	Texto     string `json:"texto"`
}
