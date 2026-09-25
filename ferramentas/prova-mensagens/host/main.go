// Host de native messaging da medição (c) da F0: responde à extensão de prova e diz de onde foi
// lançado (argumentos, confinamento Snap) e se consegue abrir os módulos PKCS#11 do catálogo medido.
// A pergunta que ele responde não é "o navegador acha o host", é "o host lançado pelo navegador
// alcança a leitora e o middleware". Não vai para release.
//
// Protocolo de native messaging: 4 bytes de tamanho na ordem nativa, depois JSON UTF-8.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/miekg/pkcs11"
)

// Os dois módulos medidos na v1 (Ubuntu e Debian); o catálogo do produto mora em nativo/.
var modulos = []string{
	"/usr/lib/libaetpkss.so.3",
	"/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
}

const limiteDeEntrada = 64 * 1024

type pedido struct {
	Op string `json:"op"`
}

type relatorioDoModulo struct {
	Caminho       string `json:"caminho"`
	Carregou      bool   `json:"carregou"`
	Erro          string `json:"erro,omitempty"`
	SlotsComToken int    `json:"slotsComToken"`
	Certificados  int    `json:"certificados"`
}

func ler(r io.Reader) ([]byte, error) {
	var tamanho uint32
	if err := binary.Read(r, binary.NativeEndian, &tamanho); err != nil {
		return nil, err
	}
	if tamanho > limiteDeEntrada {
		return nil, fmt.Errorf("mensagem de %d bytes passa do limite", tamanho)
	}
	buf := make([]byte, tamanho)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func escrever(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.NativeEndian, uint32(len(b))); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func sondar(caminho string) relatorioDoModulo {
	r := relatorioDoModulo{Caminho: caminho}
	if _, err := os.Stat(caminho); err != nil {
		r.Erro = "arquivo inacessível daqui: " + err.Error()
		return r
	}
	p := pkcs11.New(caminho)
	if p == nil {
		r.Erro = "dlopen falhou (o confinamento pode esconder as dependências do módulo)"
		return r
	}
	defer p.Destroy()
	if err := p.Initialize(); err != nil {
		r.Erro = "C_Initialize: " + err.Error()
		return r
	}
	defer func() { _ = p.Finalize() }()
	r.Carregou = true
	slots, err := p.GetSlotList(true)
	if err != nil {
		r.Erro = "C_GetSlotList: " + err.Error()
		return r
	}
	r.SlotsComToken = len(slots)
	for _, slot := range slots {
		s, err := p.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION)
		if err != nil {
			continue
		}
		if p.FindObjectsInit(s, []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE)}) == nil {
			objs, _, _ := p.FindObjects(s, 64)
			r.Certificados += len(objs)
			_ = p.FindObjectsFinal(s)
		}
		_ = p.CloseSession(s)
	}
	return r
}

func ambienteSnap() map[string]string {
	out := map[string]string{}
	for _, k := range []string{"SNAP", "SNAP_NAME", "SNAP_REVISION", "container", "FLATPAK_ID"} {
		if v, ok := os.LookupEnv(k); ok {
			out[k] = v
		}
	}
	return out
}

func main() {
	executavel, _ := os.Executable()
	for {
		msg, err := ler(os.Stdin)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Fprintln(os.Stderr, "entrada:", err)
			}
			return
		}
		var p pedido
		if err := json.Unmarshal(msg, &p); err != nil {
			_ = escrever(os.Stdout, map[string]any{"ok": false, "erro": "protocolo"})
			continue
		}
		switch p.Op {
		case "ola":
			_ = escrever(os.Stdout, map[string]any{
				"ok":         true,
				"pid":        os.Getpid(),
				"executavel": executavel,
				"argumentos": os.Args[1:],
				"plataforma": runtime.GOOS + "/" + runtime.GOARCH,
				"ambiente":   ambienteSnap(),
				"pcscd":      estadoDoPcscd(),
			})
		case "listar":
			rel := make([]relatorioDoModulo, 0, len(modulos))
			for _, m := range modulos {
				rel = append(rel, sondar(m))
			}
			_ = escrever(os.Stdout, map[string]any{"ok": true, "modulos": rel})
		default:
			_ = escrever(os.Stdout, map[string]any{"ok": false, "erro": "operacao-desconhecida"})
		}
	}
}

// estadoDoPcscd diz se o socket do pcscd é visível daqui: dentro de um Snap sem a interface pcscd,
// ele some, e nenhum middleware enxerga a leitora.
func estadoDoPcscd() string {
	for _, s := range []string{"/run/pcscd/pcscd.comm", "/var/run/pcscd/pcscd.comm"} {
		if fi, err := os.Stat(s); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return "socket visível em " + s
		} else if err != nil && !os.IsNotExist(err) {
			return "socket inacessível: " + strings.TrimSpace(err.Error())
		}
	}
	return "socket ausente"
}
