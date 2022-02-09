package main

import (
	"fmt"
	"net/http"

	"github.com/xrfang/hap"
)

func main() {
	p, err := hap.NewParser("/api/test", []hap.Param{
		{Name: "arg1", Type: "int", Memo: "int param", Default: 123},
		{Name: "arg2", Type: "string", Memo: "string param", Required: true},
		{Name: "arg3", Type: "float", Memo: "float params", Required: false},
		{Name: "help", Type: "bool", Memo: "show help"},
	})
	if err != nil {
		panic(err)
	}
	http.HandleFunc(p.Route(), func(w http.ResponseWriter, r *http.Request) {
		p.Parse(r)
		if p.Bool("help") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, p.Usage())
			return
		}
		if err := p.Error(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "arg1:", p.Integer("arg1"))
		fmt.Fprintln(w, "arg2:", p.String("arg2"))
		fmt.Fprintln(w, "arg3:", p.Floats("arg3"))
	})
	svr := http.Server{Addr: ":1234"}
	panic(svr.ListenAndServe())
}
