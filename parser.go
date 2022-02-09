package hap

import (
	"bytes"
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
	Param struct {
		Name     string `json:"name"`
		Type     string `json:"type"` //string, int, float, bool
		Default  string `json:"default"`
		Required bool   `json:"required"`
		Position uint   `json:"position"`
		Memo     string `json:"memo"`
		defval   interface{}
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
			p.opts[s.Name] = []string{s.defval.(string)}
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
			is = []int64{s.defval.(int64)}
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
			fs = []float64{s.defval.(float64)}
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
			bs = []bool{s.defval.(bool)}
		}
		p.opts[s.Name] = bs
	}
}

func (p *Parser) Parse(r *http.Request) {
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

func (p *Parser) Errors() []error {
	return p.errs
}

func (p *Parser) Strings(name string) ([]string, error) {
	switch v := p.opts[name].(type) {
	case nil:
		return nil, fmt.Errorf("parameter %q does not exist", name)
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("parameter %q is %T, not string", name, v)
	}
}

func (p *Parser) String(name string) (string, error) {
	ss, err := p.Strings(name)
	if err != nil {
		return "", err
	}
	return ss[0], nil
}

func (p *Parser) Integers(name string) ([]int64, error) {
	switch v := p.opts[name].(type) {
	case nil:
		return nil, fmt.Errorf("parameter %q does not exist", name)
	case []int64:
		return v, nil
	default:
		return nil, fmt.Errorf("parameter %q is %T, not integer", name, v)
	}
}

func (p *Parser) Integer(name string) (int64, error) {
	is, err := p.Integers(name)
	if err != nil {
		return 0, err
	}
	return is[0], nil
}

func (p *Parser) Floats(name string) ([]float64, error) {
	switch v := p.opts[name].(type) {
	case nil:
		return nil, fmt.Errorf("parameter %q does not exist", name)
	case []float64:
		return v, nil
	default:
		return nil, fmt.Errorf("parameter %q is %T, not float", name, v)
	}
}

func (p *Parser) Float(name string) (float64, error) {
	fs, err := p.Floats(name)
	if err != nil {
		return 0, err
	}
	return fs[0], nil
}

func (p *Parser) Bools(name string) ([]bool, error) {
	switch v := p.opts[name].(type) {
	case nil:
		return nil, fmt.Errorf("parameter %q does not exist", name)
	case []bool:
		return v, nil
	default:
		return nil, fmt.Errorf("parameter %q is %T, not bool", name, v)
	}
}

func (p *Parser) Bool(name string) (bool, error) {
	bs, err := p.Bools(name)
	if err != nil {
		return false, err
	}
	return bs[0], nil
}

func (p *Parser) Route() string {
	return p.path
}

func (p *Parser) Args() int {
	return len(p.args)
}

func (p *Parser) Arg(idx int) string {
	return p.args[idx]
}

func (p *Parser) Help() string {
	pargs := []string{p.path}
	var qargs []string
	for _, s := range p.pdef {
		var stub string
		if s.Required {
			stub = `<` + s.Name + `>`
		} else {
			stub = `[` + s.Name + `]`
		}
		pargs = append(pargs, stub)
	}
	if len(pargs) < 2 {
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
	uri := strings.Join(pargs, "/")
	if len(qargs) > 0 {
		uri += `?` + strings.Join(qargs, "&")
	}
	var args []map[string]interface{}
	for _, s := range append(p.pdef, p.qdef...) {
		args = append(args, map[string]interface{}{
			"name":     s.Name,
			"type":     s.Type,
			"default":  s.Default,
			"required": s.Required,
			"memo":     s.Memo,
		})
	}
	sort.Slice(args, func(i, j int) bool {
		return args[i]["name"].(string) < args[j]["name"].(string)
	})
	var bs bytes.Buffer
	je := json.NewEncoder(&bs)
	je.SetIndent("", "    ")
	je.SetEscapeHTML(false)
	je.Encode(map[string]interface{}{"uri": uri, "args": args})
	return bs.String()
}

func NewParser(route string, spec []Param) (p *Parser, err error) {
	var qdef, pdef []Param
	dup := make(map[string]bool)
	for _, s := range spec {
		if s.Name == "" {
			return nil, errors.New("empty arg name")
		}
		if dup[s.Name] {
			return nil, fmt.Errorf("arg name %q duplicated", s.Name)
		}
		dup[s.Name] = true
		t := strings.ToLower(s.Type)
		var v interface{}
		switch t {
		case "string", "":
			v = s.Default
		case "int":
			i := int(0)
			if s.Default != "" {
				i, err = strconv.Atoi(s.Default)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not a valid integer", s.Default)
				}
			}
			v = int64(i)
		case "float":
			f := float64(0)
			if s.Default != "" {
				f, err = strconv.ParseFloat(s.Default, 64)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not a valid float", s.Default)
				}
			}
			v = f
		case "bool":
			b := false
			if s.Default != "" {
				b, err = strconv.ParseBool(s.Default)
				if err != nil {
					return nil, fmt.Errorf("default value %q is not a valid bool", s.Default)
				}
			}
			v = b
		default:
			return nil, fmt.Errorf("invalid param type %q", s.Type)
		}
		s.Type = t
		s.defval = v
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
		return nil, dupErr
	}
	sort.Slice(qdef, func(i, j int) bool { return qdef[i].Name < qdef[j].Name })
	return &Parser{pdef: pdef, qdef: qdef, path: route, opts: make(map[string]interface{})}, nil
}
