package main

import (
	"fmt"
	"net/http"

	"github.com/xrfang/hap"
)

type apiTest struct{ hap.Parser }

func (at apiTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := at.Parse(r)
	if p.Bool("help") {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, at.Usage())
		return
	}
	if err := p.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, "%+v\n", p.ExportAll())
}

func main() {
	var at apiTest
	err := at.Init("/api/test", []hap.Param{
		{Name: "arg0", Type: "int", Memo: "positional argument", Position: 1, Required: true},
		{Name: "arg1", Type: "int", Memo: "int param", Default: 123},
		{Name: "arg2", Type: "string", Memo: "string param", Required: true,
			Check: func(i interface{}) error {
				if i == nil {
					return fmt.Errorf("allowed: `val1` or `val2`")
				}
				switch i.(string) {
				case "val1", "val2":
					return nil
				default:
					return fmt.Errorf("'%v'` is not valid for arg2", i)
				}
			}},
		{Name: "arg3", Type: "float", Memo: "float params", Required: false},
		{Name: "garg", Memo: "get param", Required: true, Methods: "GET"},
		{Name: "parg", Memo: "post/put/patch param", Required: true, Methods: "POST,PUT,PATCH"},
		{Name: "help", Type: "bool", Memo: "show help"},
		{Memo: "example of using HAP"},
	})
	if err != nil {
		panic(err)
	}
	hap.Register(at)
	for _, m := range hap.Manifest() {
		fmt.Printf("%s\t%s\n", m.Route, m.Purpose)
	}
	svr := http.Server{Addr: ":1234"}
	panic(svr.ListenAndServe())
}
