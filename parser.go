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
	HandlerInfo struct {
		Route   string `json:"route"`
		Purpose string `json:"purpose"`
	}
	Validator func(interface{}) error
	Handler   interface {
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
		Memo     string
	}
	Parser struct {
		qdef []Param //query parameters
		pdef []Param //positional (path) parameters
		opts map[string]interface{}
		help string
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
		p.errs = append(p.errs, fmt.Errorf("missing '%s'", s.Name))
		return
	}
	switch s.Type {
	case "string":
		if len(vals) == 0 {
			p.opts[s.Name] = []string{s.Default.(string)}
		} else {
			if s.Check != nil {
				for _, v := range vals {
					if err := s.Check(v); err != nil {
						p.errs = append(p.errs, err)
					}
				}
			}
			p.opts[s.Name] = vals
		}
	case "int":
		var is []int64
		for _, v := range vals {
			i, err := strconv.Atoi(v)
			if err != nil {
				p.errs = append(p.errs, fmt.Errorf("'%s' is not an integer (arg:%s)", v, s.Name))
				return
			}
			is = append(is, int64(i))
		}
		if len(is) == 0 {
			is = []int64{s.Default.(int64)}
		} else if s.Check != nil {
			for _, v := range is {
				if err := s.Check(v); err != nil {
					p.errs = append(p.errs, err)
				}
			}
		}
		p.opts[s.Name] = is
	case "float":
		var fs []float64
		for _, v := range vals {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				p.errs = append(p.errs, fmt.Errorf("'%s' is not a float (arg:%s)", v, s.Name))
				return
			}
			fs = append(fs, f)
		}
		if len(fs) == 0 {
			fs = []float64{s.Default.(float64)}
		} else if s.Check != nil {
			for _, v := range fs {
				if err := s.Check(v); err != nil {
					p.errs = append(p.errs, err)
				}
			}
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
					p.errs = append(p.errs, fmt.Errorf("'%s' is not a bool (arg:%s)", v, s.Name))
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
	if sfx := r.URL.Path[len(p.path):]; len(sfx) > 1 {
		p.args = strings.Split(sfx[1:], "/")
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
	return p.Spec()
}

func (p Parser) Strings(name string) []string {
	switch v := p.opts[name].(type) {
	case nil:
		return nil
	case []string:
		return v
	default:
		panic(fmt.Errorf("parameter '%s' is %T, not string", name, v))
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
		panic(fmt.Errorf("parameter '%s' is %T, not integer", name, v))
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
		panic(fmt.Errorf("parameter '%s' is %T, not float", name, v))
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
		panic(fmt.Errorf("parameter '%s' is %T, not bool", name, v))
	}
}

func (p Parser) Bool(name string) bool {
	bs := p.Bools(name)
	if len(bs) == 0 {
		return false
	}
	return bs[0]
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

func (p Parser) Args() int {
	return len(p.args)
}

func (p Parser) Arg(idx int) string {
	return p.args[idx]
}

func (p Parser) Spec() Error {
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
		if s.Check != nil {
			a["check"] = s.Check(nil).Error()
		}
		args = append(args, a)
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i]["name"].(string) < args[j]["name"].(string)
	})
	return Error{
		errs: p.errs,
		help: p.help,
		path: uri,
		args: args,
	}
}

func (p Parser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (p Parser) Usage() string {
	return p.Spec().Error()
}

func (p *Parser) Init(route string, spec []Param) error {
	var qdef, pdef []Param
	dup := make(map[string]bool)
specParse:
	for _, s := range spec {
		if dup[s.Name] {
			return fmt.Errorf("arg name '%s' duplicated", s.Name)
		}
		dup[s.Name] = true
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
			if s.Name == "" && t == "" {
				p.help = s.Memo
				continue specParse
			}
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
		if s.Name == "" {
			return errors.New("empty arg name")
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
	p.opts = make(map[string]interface{})
	return nil
}

var handlers map[string]string

func Register(h Handler, mx ...*http.ServeMux) {
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
