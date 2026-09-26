//go:build windows

package pcsc

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"
	"unsafe"

	win "golang.org/x/sys/windows"
)

// O PC/SC do Windows é o `winscard.dll`, do sistema: sem cgo, pelas funções `W` (nomes em UTF-16).
// `NewLazySystemDLL` só carrega da pasta do sistema.
var (
	winscard              = win.NewLazySystemDLL("winscard.dll")
	procEstabelecer       = winscard.NewProc("SCardEstablishContext")
	procListarLeitoras    = winscard.NewProc("SCardListReadersW")
	procEstadoDasLeitoras = winscard.NewProc("SCardGetStatusChangeW")
	procLiberarContexto   = winscard.NewProc("SCardReleaseContext")
)

const (
	escopoDoUsuario    = 0  // SCARD_SCOPE_USER
	atrMaximoPelaNorma = 33 // SCARD_ATR_LENGTH (ISO/IEC 7816-3)
	// tentativasDeListar: a lista pode mudar entre o pedido do tamanho e o da lista (uma leitora
	// conectada no meio), e aí o PC/SC responde que o espaço não bastou.
	tentativasDeListar = 3
)

// estadoDaLeitora é o SCARD_READERSTATEW (winscard.h). O teste de cabeçalho
// (`cabecalho_windows.go`, tag `pcsc_cabecalho`) confere tamanho e deslocamentos contra o SDK.
type estadoDaLeitora struct {
	leitora    *uint16
	usuario    uintptr
	atual      uint32
	evento     uint32
	tamanhoATR uint32
	atr        [36]byte
}

func falha(codigo uint32) Resultado {
	return Resultado{Estado: estadoDoCodigo(codigo), Detalhe: fmt.Sprintf("0x%08X", codigo), Leitoras: []Leitora{}}
}

// chamar faz a chamada e devolve o LONG do PC/SC como código (zero é SCARD_S_SUCCESS).
func chamar(p *win.LazyProc, args ...uintptr) uint32 {
	r, _, _ := p.Call(args...)
	return uint32(r)
}

// listarNomes pede o tamanho da lista (em caracteres), aloca e lê; se a lista cresceu no meio
// (SCARD_E_INSUFFICIENT_BUFFER), tenta de novo. Lista acima do teto é falha, e não "nenhuma
// leitora". O que o PC/SC diz que escreveu nunca é lido além do que foi alocado.
func listarNomes(ctx uintptr) ([]string, *Resultado) {
	for range tentativasDeListar {
		var tamanho uint32
		if rv := chamar(procListarLeitoras, ctx, 0, 0, uintptr(unsafe.Pointer(&tamanho))); rv != 0 {
			r := falha(rv)
			return nil, &r
		}
		if tamanho == 0 {
			return nil, nil
		}
		if tamanho > tamanhoMaximoDaLista {
			return nil, &Resultado{Estado: EstadoFalhou, Detalhe: "lista de leitoras acima do teto", Leitoras: []Leitora{}}
		}
		alocado := tamanho
		buf := make([]uint16, alocado+1)
		rv := chamar(procListarLeitoras, ctx, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&tamanho)))
		if rv == codigoBufferPequeno {
			continue
		}
		if rv != 0 {
			r := falha(rv)
			return nil, &r
		}
		lido := min(tamanho, alocado)
		return nomesDaLista([]byte(string(utf16.Decode(buf[:lido])))), nil
	}
	return nil, &Resultado{Estado: EstadoFalhou, Detalhe: "a lista de leitoras não parou de mudar", Leitoras: []Leitora{}}
}

// Consultar lê as leitoras e o estado de cada uma, sem conectar ao cartão. Pode bloquear enquanto o
// serviço Cartão Inteligente responde: quem chama põe prazo.
func Consultar() Resultado {
	if err := winscard.Load(); err != nil {
		return Resultado{Estado: EstadoSemBiblioteca, Leitoras: []Leitora{}}
	}
	var ctx uintptr
	if rv := chamar(procEstabelecer, escopoDoUsuario, 0, 0, uintptr(unsafe.Pointer(&ctx))); rv != 0 {
		return falha(rv)
	}
	defer chamar(procLiberarContexto, ctx)
	nomes, recusa := listarNomes(ctx)
	if recusa != nil {
		return *recusa
	}
	if len(nomes) == 0 {
		return Resultado{Estado: EstadoSemLeitora, Leitoras: []Leitora{}}
	}
	// Os nomes em UTF-16 ficam vivos pelo ponteiro tipado dentro de `estados`.
	estados := make([]estadoDaLeitora, len(nomes))
	for i, nome := range nomes {
		p, err := win.UTF16PtrFromString(nome)
		if err != nil {
			return Resultado{Estado: EstadoFalhou, Detalhe: "nome de leitora ilegível", Leitoras: []Leitora{}}
		}
		estados[i].leitora = p
	}
	// Tempo zero e estado de partida "desconhecido": a chamada volta na hora com o estado atual de cada
	// leitora e o ATR do cartão.
	if rv := chamar(procEstadoDasLeitoras, ctx, 0, uintptr(unsafe.Pointer(&estados[0])), uintptr(len(estados))); rv != 0 && rv != codigoTempoEsgotado {
		return falha(rv)
	}
	saida := Resultado{Estado: EstadoOk, Leitoras: make([]Leitora, 0, len(nomes))}
	for i, nome := range nomes {
		e := estados[i]
		l := Leitora{Nome: nome, ComCartao: e.evento&estadoPresente != 0, Mudo: e.evento&estadoMudo != 0}
		if l.ComCartao && e.tamanhoATR > 0 && e.tamanhoATR <= atrMaximoPelaNorma {
			l.ATR = strings.ToUpper(hex.EncodeToString(e.atr[:e.tamanhoATR]))
		}
		saida.Leitoras = append(saida.Leitoras, l)
	}
	return saida
}
