//go:build windows

package windows

import (
	win "golang.org/x/sys/windows"

	"github.com/Gottbrok/assinador/nativo/internal/diagnostico"
)

// EstadoDoServicoDePropagacao consulta, só para leitura, o serviço de Propagação de Certificados
// (`CertPropSvc`), que põe o certificado do cartão no repositório do usuário. O acesso pedido é o
// mínimo (conectar ao gerenciador e ler o estado): funciona sem administrador.
func EstadoDoServicoDePropagacao() string {
	gerenciador, err := win.OpenSCManager(nil, nil, win.SC_MANAGER_CONNECT)
	if err != nil {
		return diagnostico.ServicoDesconhecido
	}
	defer win.CloseServiceHandle(gerenciador)
	nome, _ := win.UTF16PtrFromString("CertPropSvc")
	servico, err := win.OpenService(gerenciador, nome, win.SERVICE_QUERY_STATUS)
	if err != nil {
		return diagnostico.ServicoDesconhecido
	}
	defer win.CloseServiceHandle(servico)
	var estado win.SERVICE_STATUS
	if err := win.QueryServiceStatus(servico, &estado); err != nil {
		return diagnostico.ServicoDesconhecido
	}
	switch estado.CurrentState {
	case win.SERVICE_RUNNING, win.SERVICE_START_PENDING:
		return diagnostico.ServicoRodando
	}
	return diagnostico.ServicoParado
}
