package hap

import (
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
)

type (
	HandlerInfo struct {
		Route   string `json:"route"`
		Purpose string `json:"purpose"`
	}
	Validator func(interface{}) error
	Handler   interface {
		Ready() bool
		Routes() []string
		Purpose() string
		http.Handler
	}
	Param struct {
		Name     string
		Type     string //string, int, float, bool
		Default  interface{}
		Required bool
		Check    Validator
		Position uint
		Methods  string //CSV格式的HTTP方法列表，默认为空表示全部允许
		Memo     string
		verbs    HttpMethods
	}
	Parser struct {
		init bool
		qdef []Param //query parameters
		pdef []Param //positional (path) parameters
		help string
		path string
	}
	Result struct {
		pars *Parser
		args []string
		errs []error
		has  map[string]bool
		opts map[string]interface{}
	}
)

func (r *Result) parse(vals []string, s Param) {
	if len(vals) == 0 && s.Required {
		r.errs = append(r.errs, fmt.Errorf("missing '%s'", s.Name))
		return
	}
	switch s.Type {
	case "string":
		if len(vals) == 0 {
			r.opts[s.Name] = []string{s.Default.(string)}
		} else {
			if s.Check != nil {
				for _, v := range vals {
					if err := s.Check(v); err != nil {
						r.errs = append(r.errs, err)
					}
				}
			}
			r.opts[s.Name] = vals
		}
	case "int":
		var is []int64
		var base int
		for _, v := range vals {
			switch {
			case strings.HasPrefix(v, "0x"), strings.HasPrefix(v, "0X"):
				base = 16
			case strings.HasPrefix(v, "0"):
				base = 8
			default:
				base = 10
			}
			i, err := strconv.ParseInt(v, base, 64)
			if err != nil {
				r.errs = append(r.errs, fmt.Errorf("'%s' is not an integer (arg:%s)", v, s.Name))
				return
			}
			is = append(is, int64(i))
		}
		if len(is) == 0 {
			is = []int64{s.Default.(int64)}
		} else if s.Check != nil {
			for _, v := range is {
				if err := s.Check(v); err != nil {
					r.errs = append(r.errs, err)
				}
			}
		}
		r.opts[s.Name] = is
	case "float":
		var fs []float64
		for _, v := range vals {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				r.errs = append(r.errs, fmt.Errorf("'%s' is not a float (arg:%s)", v, s.Name))
				return
			}
			fs = append(fs, f)
		}
		if len(fs) == 0 {
			fs = []float64{s.Default.(float64)}
		} else if s.Check != nil {
			for _, v := range fs {
				if err := s.Check(v); err != nil {
					r.errs = append(r.errs, err)
				}
			}
		}
		r.opts[s.Name] = fs
	case "bool":
		var bs []bool
		for _, v := range vals {
			b := true
			var err error
			if v != "" {
				b, err = strconv.ParseBool(v)
				if err != nil {
					r.errs = append(r.errs, fmt.Errorf("'%s' is not a bool (arg:%s)", v, s.Name))
					return
				}
			}
			bs = append(bs, b)
		}
		if len(bs) == 0 {
			bs = []bool{s.Default.(bool)}
		}
		r.opts[s.Name] = bs
	}
}

func (p *Parser) Parse(req *http.Request) (r Result) {
	r.pars = p
	r.opts = make(map[string]interface{})
	r.has = make(map[string]bool)
	if sfx := req.URL.Path[len(p.path):]; len(sfx) > 1 {
		r.args = strings.Split(sfx[1:], "/")
	}
	for i, s := range p.pdef {
		if !s.verbs.Contains(HttpMethod(req.Method)) {
			continue
		}
		var arg []string
		if i < len(r.args) {
			arg = []string{r.args[i]}
		}
		r.parse(arg, s)
	}
	vs, err := args(req)
	if err != nil {
		r.errs = append(r.errs, err)
		return
	}
	for _, s := range p.qdef {
		if !s.verbs.Contains(HttpMethod(req.Method)) {
			continue
		}
		r.has[s.Name] = vs.Has(s.Name)
		r.parse(vs[s.Name], s)
	}
	return
}

func (r Result) Error() error {
	if len(r.errs) == 0 {
		return nil
	}
	return r.pars.Spec(r.errs)
}

func (r Result) Errs() []error {
	return r.errs
}

func (r Result) Has(name string) bool {
	return r.has[name]
}

func (r Result) Strings(name string) []string {
	switch v := r.opts[name].(type) {
	case nil:
		return nil
	case []string:
		return v
	default:
		panic(fmt.Errorf("parameter '%s' is %T, not string", name, v))
	}
}

func (r Result) String(name string) string {
	ss := r.Strings(name)
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

func (r Result) Integers(name string) []int64 {
	switch v := r.opts[name].(type) {
	case nil:
		return nil
	case []int64:
		return v
	default:
		panic(fmt.Errorf("parameter '%s' is %T, not integer", name, v))
	}
}

func (r Result) Integer(name string) int64 {
	is := r.Integers(name)
	if len(is) == 0 {
		return 0
	}
	return is[0]
}

func (r Result) Floats(name string) []float64 {
	switch v := r.opts[name].(type) {
	case nil:
		return nil
	case []float64:
		return v
	default:
		panic(fmt.Errorf("parameter '%s' is %T, not float", name, v))
	}
}

func (r Result) Float(name string) float64 {
	fs := r.Floats(name)
	if len(fs) == 0 {
		return 0
	}
	return fs[0]
}

func (r Result) Bools(name string) []bool {
	switch v := r.opts[name].(type) {
	case nil:
		return nil
	case []bool:
		return v
	default:
		panic(fmt.Errorf("parameter '%s' is %T, not bool", name, v))
	}
}

func (r Result) Bool(name string) bool {
	bs := r.Bools(name)
	if len(bs) == 0 {
		return false
	}
	return bs[0]
}

func (r Result) Values(name string) []interface{} {
	if !r.Has(name) {
		return nil
	}
	var vals []interface{}
	switch vs := r.opts[name].(type) {
	case []string:
		for _, v := range vs {
			vals = append(vals, v)
		}
	case []int64:
		for _, v := range vs {
			vals = append(vals, v)
		}
	case []float64:
		for _, v := range vs {
			vals = append(vals, v)
		}
	case []bool:
		for _, v := range vs {
			vals = append(vals, v)
		}
	}
	return vals
}

func (p Parser) Purpose() string {
	return p.help
}

func (p Parser) Routes() []string {
	if p.path == "" {
		return []string{"/"}
	}
	return []string{p.path, p.path + "/"}
}

func (r Result) Args() int {
	return len(r.args)
}

func (r Result) Arg(idx int) string {
	return r.args[idx]
}

func (p Parser) Spec(errs []error) Error {
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
			"position": s.Position,
			"memo":     s.Memo,
		}
		if len(s.verbs) > 0 {
			a["methods"] = s.verbs.String()
		}
		if !s.Required {
			a["default"] = s.Default
		}
		if s.Check != nil {
			if err := s.Check(nil); err != nil {
				a["check"] = err.Error()
			}
		}
		args = append(args, a)
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i]["name"].(string) < args[j]["name"].(string)
	})
	return Error{
		errs: errs,
		help: p.help,
		path: uri,
		args: args,
	}
}

func (p Parser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (p Parser) Usage() string {
	return p.Spec(nil).Error()
}

func (r Result) ExportAll() map[string]interface{} {
	return r.opts
}

func (r Result) Export() map[string]interface{} {
	args := make(map[string]interface{})
	for k, v := range r.opts {
		switch d := v.(type) {
		case []string:
			if len(d) > 0 {
				args[k] = d[0]
			}
		case []int64:
			if len(d) > 0 {
				args[k] = d[0]
			}
		case []float64:
			if len(d) > 0 {
				args[k] = d[0]
			}
		case []bool:
			if len(d) > 0 {
				args[k] = d[0]
			}
		}
	}
	return args
}

func (p *Parser) Init(route string, spec []Param) error {
	var qdef, pdef []Param
	dup := make(map[string]bool)
	for _, s := range spec {
		if dup[s.Name] {
			return fmt.Errorf("arg name '%s' duplicated", s.Name)
		}
		dup[s.Name] = true
		if s.Name == "" {
			p.help = s.Memo
			continue
		}
		for _, m := range strings.Split(s.Methods, ",") {
			if m = strings.TrimSpace(m); m == "" {
				continue
			}
			if HttpMethod(m).Value() == 0 {
				return fmt.Errorf("invalid HTTP method %q (arg:%s)", m, s.Name)
			}
			s.verbs = append(s.verbs, HttpMethod(m))
		}
		t := strings.ToLower(s.Type)
		var v interface{}
		switch t {
		case "string", "":
			switch r := s.Default.(type) {
			case string:
				v = r
			case nil:
				v = ""
			default:
				return fmt.Errorf("default value of '%s' must be string (given: %T)", s.Name, s.Default)
			}
			t = "string"
		case "int":
			switch i := s.Default.(type) {
			case int64:
				v = i
			case int:
				v = int64(i)
			case nil:
				v = int64(0)
			default:
				return fmt.Errorf("default value of '%s' must be int or int64 (given: %T)",
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
				return fmt.Errorf("default value of '%s' must be float (given: %T)", s.Name, s.Default)
			}
		case "bool":
			switch b := s.Default.(type) {
			case bool:
				v = b
			case nil:
				v = false
			default:
				return fmt.Errorf("default value of '%s' must be bool (given: %T)", s.Name, s.Default)
			}
		default:
			return fmt.Errorf("invalid param type '%s'", s.Type)
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
			dupErr = fmt.Errorf("same position ('%s', '%s')", pi.Name, pj.Name)
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
	if strings.HasSuffix(p.path, "/") {
		p.path = route[:len(p.path)-1]
	}
	p.init = true
	return nil
}

func (p Parser) Ready() bool {
	return p.init
}

var handlers map[string]string

func Register(h Handler, mx ...*http.ServeMux) {
	if !h.Ready() {
		panic(fmt.Errorf("hap: handler %T not initialized", h))
	}
	if len(mx) == 0 {
		mx = []*http.ServeMux{http.DefaultServeMux}
	}
	routes := h.Routes()
	for _, r := range routes {
		for _, x := range mx {
			x.Handle(r, h)
		}
	}
	handlers[routes[0]] = h.Purpose()
}

func Manifest() []HandlerInfo {
	var his []HandlerInfo
	for r, p := range handlers {
		his = append(his, HandlerInfo{r, p})
	}
	sort.Slice(his, func(i, j int) bool { return his[i].Route < his[j].Route })
	return his
}

func init() {
	handlers = make(map[string]string)
}
