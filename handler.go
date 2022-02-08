package hap

import "net/http"

type (
	RequestProc func(p *Parser, w http.ResponseWriter, r *http.Request)
	handler     struct {
		argp *Parser
		proc RequestProc
	}
)

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.proc == nil {
		http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
		return
	}
	if h.argp != nil {
		h.argp.parse(r)
	}
	h.proc(h.argp, w, r)
}

func (h *handler) Register(mux ...*http.ServeMux) {
	if mux == nil {
		mux = []*http.ServeMux{http.DefaultServeMux}
	}
	mux[0].Handle(h.argp.Route(), h)
}

func NewHandler(p *Parser, s RequestProc) *handler {
	return &handler{p, s}
}
