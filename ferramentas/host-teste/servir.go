package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"
)

// servir é a página de teste da EXTENSÃO (F3): serve a página (`extensao/e2e/pagina`), que fala o
// protocolo com a extensão como a biblioteca fala, e faz o papel do servidor que prepara a
// assinatura: emite o bilhete com a chave dev local (`POST /bilhete`) e confere a assinatura que
// voltou contra o certificado (`POST /conferir`).
//
// Escuta só em 127.0.0.1, e a origem é `http://localhost:<porta>`: é o endereço que o build `dev` da
// extensão e do programa aceita. A primeira linha da saída é `servindo em <origem>`, que o teste
// ponta a ponta lê.
func servir(args []string) error {
	fs := flag.NewFlagSet("servir", flag.ExitOnError)
	porta := fs.Int("porta", 8787, "porta em localhost (0 escolhe uma livre)")
	pagina := fs.String("pagina", filepath.Join("extensao", "e2e", "pagina"), "a pasta da página de teste")
	_ = fs.Parse(args)
	chave, err := lerChavePrivada()
	if err != nil {
		return err
	}
	ouvinte, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *porta))
	if err != nil {
		return err
	}
	origem := fmt.Sprintf("http://localhost:%d", ouvinte.Addr().(*net.TCPAddr).Port)
	fmt.Println("servindo em", origem)
	srv := &http.Server{Handler: rotasDaPagina(chave, origem, *pagina), ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ouvinte)
}

var formaHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

const documentoPadrao = "Documento de teste do Assinador"

// rotasDaPagina monta as rotas; separado de `servir` para o teste.
func rotasDaPagina(chave *ecdsa.PrivateKey, origem, pagina string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServer(http.Dir(pagina)))
	mux.HandleFunc("POST /bilhete", func(w http.ResponseWriter, r *http.Request) {
		var corpo struct {
			Ref       string `json:"ref"`
			Digest    string `json:"digest"`
			Documento string `json:"documento"`
		}
		if !lerCorpo(w, r, &corpo) {
			return
		}
		if !formaHex64.MatchString(corpo.Ref) || !formaHex64.MatchString(corpo.Digest) {
			responder(w, http.StatusBadRequest, map[string]string{"erro": "ref e digest são SHA-256 em hexadecimal minúsculo"})
			return
		}
		doc := corpo.Documento
		if doc == "" {
			doc = documentoPadrao
		}
		if !textoDeBilhete(doc, 200) {
			responder(w, http.StatusBadRequest, map[string]string{"erro": "documento com até 200 caracteres e sem caractere de controle"})
			return
		}
		jws, err := bilhete(chave, origem, corpo.Digest, corpo.Ref, doc)
		if err != nil {
			responder(w, http.StatusInternalServerError, map[string]string{"erro": err.Error()})
			return
		}
		responder(w, http.StatusOK, map[string]string{"bilhete": jws})
	})
	mux.HandleFunc("POST /conferir", func(w http.ResponseWriter, r *http.Request) {
		var corpo struct {
			Der        string `json:"der"`
			Digest     string `json:"digest"`
			Assinatura string `json:"assinatura"`
		}
		if !lerCorpo(w, r, &corpo) {
			return
		}
		if err := conferirAssinatura(corpo.Der, corpo.Digest, corpo.Assinatura); err != nil {
			responder(w, http.StatusOK, map[string]any{"confere": false, "motivo": err.Error()})
			return
		}
		responder(w, http.StatusOK, map[string]any{"confere": true})
	})
	return semCache(mux)
}

// conferirAssinatura confere a assinatura RSA PKCS#1 v1.5 (DigestInfo SHA-256) do `digest` contra a
// chave pública do certificado: o que o Confidata faz ao completar o CMS.
func conferirAssinatura(derB64, digestHex, assinaturaB64 string) error {
	der, err := base64.StdEncoding.DecodeString(derB64)
	if err != nil {
		return errors.New("der fora de base64")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return errors.New("certificado ilegível")
	}
	publica, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("o certificado não é RSA")
	}
	if !formaHex64.MatchString(digestHex) {
		return errors.New("digest fora da forma")
	}
	digest, _ := hex.DecodeString(digestHex)
	assinatura, err := base64.StdEncoding.DecodeString(assinaturaB64)
	if err != nil {
		return errors.New("assinatura fora de base64")
	}
	if err := rsa.VerifyPKCS1v15(publica, crypto.SHA256, digest, assinatura); err != nil {
		return fmt.Errorf("a assinatura não confere com o certificado: %w", err)
	}
	return nil
}

// textoDeBilhete: até `limite` pontos de código e nenhum caractere de controle (o que o emissor
// garante antes de assinar).
func textoDeBilhete(s string, limite int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > limite {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func lerCorpo(w http.ResponseWriter, r *http.Request, destino any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(destino); err != nil {
		responder(w, http.StatusBadRequest, map[string]string{"erro": "corpo fora da forma"})
		return false
	}
	return true
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

func semCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
