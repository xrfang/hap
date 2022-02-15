package hap

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

type (
	Handler interface {
		Routes() []string
		http.Handler
	}
	Param struct {
		Name     string
		Type     string //string, int, float, bool
		Default  interface{}
		Required bool
		Position uint
		Memo     string
	}
	Parser struct {
		qdef []Param //query parameters
		pdef []Param //positional (path) parameters
		opts map[string]interface{}
		args []string
		path string
		errs []error
	}
)

//parse query string in G-P-C order, i.e. GET (query string) has highest priority,
//POST (body) follows, and COOKIE has lowest priority
func args(r *http.Request) (url.Values, error) {
	vs := make(url.Values)
	for _, c := range r.Cookies() {
		vs[c.Name] = []string{c.Value}
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		switch ct {
		case "application/json":
			var kv map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&kv)
			if err != nil {
				return nil, err
			}
			for k, v := range kv {
				switch v := v.(type) {
				case string:
					vs[k] = []string{v}
				case []string:
					vs[k] = v
				default:
					vs[k] = []string{fmt.Sprintf("%v", v)}
				}
			}
		case "multipart/form-data":
			err := r.ParseMultipartForm(10 << 20)
			if err != nil {
				return nil, err
			}
			fallthrough
		default:
			err := r.ParseForm()
			if err != nil {
				return nil, err
			}
			for k, v := range r.Form {
				vs[k] = v
			}
		}
	}
	for k, v := range r.URL.Query() {
		vs[k] = v
	}
	pq, _ := url.ParseQuery(r.URL.Path[1:])
	for k, v := range pq {
		k = path.Base(k)
		if !vs.Has(k) {
			vs[k] = v
		}
	}
	return vs, nil
}

func (p *Parser) parse(vals []string, s Param) {
	if len(vals) == 0 && s.Required {
		p.errs = append(p.errs, fmt.Errorf("missing %q", s.Name))
		return
	}
	switch s.Type {
	case "string":
		if len(vals) == 0 {
			p.opts[s.Name] = []string{s.Default.(string)}
		} else {
			p.opts[s.Name] = vals
		}
	case "int":
		var is []int64
		for _, v := range vals {
			i, err := strconv.Atoi(v)
			if err != nil {
				p.errs = append(p.errs, fmt.Errorf("%q is not an integer (arg:%s)", v, s.Name))
				return
			}
			is = append(is, int64(i))
		}
		if len(is) == 0 {
			is = []int64{s.Default.(int64)}
		}
		p.opts[s.Name] = is
	case "float":
		var fs []float64
		for _, v := range vals {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				p.errs = append(p.errs, fmt.Errorf("%q is not a float (arg:%s)", v, s.Name))
				return
			}
			fs = append(fs, f)
		}
		if len(fs) == 0 {
			fs = []float64{s.Default.(float64)}
		}
		p.opts[s.Name] = fs
	case "bool":
		var bs []bool
		for _, v := range vals {
			b := true
			var err error
			if v != "" {
				b, err = strconv.ParseBool(v)
				if err != nil {
					p.errs = append(p.errs, fmt.Errorf("%q is not a bool (arg:%s)", v, s.Name))
					return
				}
			}
			bs = append(bs, b)
		}
		if len(bs) == 0 {
			bs = []bool{s.Default.(bool)}
		}
		p.opts[s.Name] = bs
	}
}

func (p *Parser) Parse(r *http.Request) {
	p.errs = nil
	if sfx := r.URL.Path[len(p.path):]; len(sfx) > 0 {
		p.args = strings.Split(sfx, "/")
	}
	for i, s := range p.pdef {
		var arg []string
		if i < len(p.args) {
			arg = []string{p.args[i]}
		}
		p.parse(arg, s)
	}
	vs, err := args(r)
	if err != nil {
		p.errs = append(p.errs, err)
		return
	}
	for _, s := range p.qdef {
		v := vs[s.Name]
		p.parse(v, s)
	}
}

func (p Parser) Error() error {
	if len(p.errs) == 0 {
		return nil
	}
	return p.spec()
}

func (p Parser) Strings(name string) []string {
	switch v := p.opts[name].(type) {
	case nil:
		return nil
	case []string:
		return v
	default:
		panic(fmt.Errorf("parameter %q is %T, not string", name, v))
	}
}

func (p Parser) String(name string) string {
	ss := p.Strings(name)
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

func (p Parser) Integers(name string) []int64 {
	switch v := p.opts[name].(type) {
	case nil:
		return nil
	case []int64:
		return v
	default:
		panic(fmt.Errorf("parameter %q is %T, not integer", name, v))
	}
}

func (p Parser) Integer(name string) int64 {
	is := p.Integers(name)
	if len(is) == 0 {
		return 0
	}
	return is[0]
}

func (p Parser) Floats(name string) []float64 {
	switch v := p.opts[name].(type) {
	case nil:
		return nil
	case []float64:
		return v
	default:
		panic(fmt.Errorf("parameter %q is %T, not float", name, v))
	}
}

func (p Parser) Float(name string) float64 {
	fs := p.Floats(name)
	if len(fs) == 0 {
		return 0
	}
	return fs[0]
}

func (p Parser) Bools(name string) []bool {
	switch v := p.opts[name].(type) {
	case nil:
		return nil
	case []bool:
		return v
	default:
		panic(fmt.Errorf("parameter %q is %T, not bool", name, v))
	}
}

func (p Parser) Bool(name string) bool {
	bs := p.Bools(name)
	if len(bs) == 0 {
		return false
	}
	return bs[0]
}

func (p Parser) Routes() []string {
	rs := []string{p.path}
	if strings.HasSuffix(p.path, "/") {
		rs = append(rs, p.path[:len(p.path)-1])
	} else {
		rs = append(rs, p.path+"/")
	}
	return rs
}

func (p Parser) Args() int {
	return len(p.args)
}

func (p Parser) Arg(idx int) string {
	return p.args[idx]
}

func (p Parser) spec() Error {
	var pargs, qargs []string
	for _, s := range p.pdef {
		var stub string
		if s.Required {
			stub = `<` + s.Name + `>`
		} else {
			stub = `[` + s.Name + `]`
		}
		pargs = append(pargs, stub)
	}
	if len(pargs) == 0 {
		pargs = append(pargs, `[arguments]`)
	}
	for _, s := range p.qdef {
		var stub string
		if s.Required {
			stub = `<` + s.Name + `>`
		} else {
			stub = `[` + s.Name + `]`
		}
		qargs = append(qargs, stub)
	}
	uri := path.Join(p.path, strings.Join(pargs, "/"))
	if len(qargs) > 0 {
		uri += `?` + strings.Join(qargs, "&")
	}
	var args []map[string]interface{}
	for _, s := range append(p.pdef, p.qdef...) {
		a := map[string]interface{}{
			"name":     s.Name,
			"type":     s.Type,
			"required": s.Required,
			"memo":     s.Memo,
		}
		if !s.Required {
			a["default"] = s.Default
		}
		args = append(args, a)
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i]["name"].(string) < args[j]["name"].(string)
	})
	return Error{
		errs: p.errs,
		path: uri,
		args: args,
	}
}

func (p Parser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (p Parser) Usage() string {
	return p.spec().Error()
}

func (p *Parser) Init(route string, spec []Param) error {
	var qdef, pdef []Param
	dup := make(map[string]bool)
	for _, s := range spec {
		if s.Name == "" {
			return errors.New("empty arg name")
		}
		if dup[s.Name] {
			return fmt.Errorf("arg name %q duplicated", s.Name)
		}
		dup[s.Name] = true
		t := strings.ToLower(s.Type)
		var v interface{}
		switch t {
		case "string", "":
			v = s.Default
		case "int":
			switch i := s.Default.(type) {
			case int64:
				v = i
			case int:
				v = int64(i)
			case nil:
				v = int64(0)
			default:
				return fmt.Errorf("default value of %q must be int or int64 (given: %T)",
					s.Name, s.Default)
			}
		case "float":
			switch f := s.Default.(type) {
			case float32:
				v = float64(f)
			case float64:
				v = f
			case nil:
				v = float64(0)
			default:
				return fmt.Errorf("default value of %q must be float (given: %T)", s.Name, s.Default)
			}
		case "bool":
			switch b := s.Default.(type) {
			case bool:
				v = b
			case nil:
				v = false
			default:
				return fmt.Errorf("default value of %q must be bool (given: %T)", s.Name, s.Default)
			}
		default:
			return fmt.Errorf("invalid param type %q", s.Type)
		}
		s.Type = t
		s.Default = v
		if s.Position > 0 {
			pdef = append(pdef, s)
		} else {
			qdef = append(qdef, s)
		}
	}
	var dupErr error
	sort.Slice(pdef, func(i, j int) bool {
		pi := pdef[i]
		pj := pdef[j]
		if pi.Position == pj.Position {
			dupErr = fmt.Errorf("same position (%q, %q)", pi.Name, pj.Name)
			return false
		}
		return pi.Position < pj.Position
	})
	if dupErr != nil {
		return dupErr
	}
	sort.Slice(qdef, func(i, j int) bool { return qdef[i].Name < qdef[j].Name })
	p.pdef = pdef
	p.qdef = qdef
	p.path = route
	p.opts = make(map[string]interface{})
	return nil
}

func Register(h Handler, mx ...*http.ServeMux) {
	if len(mx) == 0 {
		mx = []*http.ServeMux{http.DefaultServeMux}
	}
	for _, r := range h.Routes() {
		for _, x := range mx {
			x.Handle(r, h)
		}
	}
}
