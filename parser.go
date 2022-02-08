package hap

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type (
	Param struct {
		Name     string `json:"name"`
		Type     string `json:"type"` //string, int, float, bool
		Default  string `json:"default"`
		Required bool   `json:"required"`
		Memo     string `json:"memo"`
		defval   interface{}
	}
	Parser struct {
		spec []Param
		opts map[string]interface{}
		args []string
		path string
		err  error
	}
)

func assert(e error) {
	if e != nil {
		panic(e)
	}
}

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

func (p *Parser) parse(r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			p.err = e.(error)
		}
	}()
	if sfx := r.URL.Path[len(p.path):]; len(sfx) > 0 {
		p.args = strings.Split(sfx, "/")
	}
	vs, err := args(r)
	assert(err)
	for _, s := range p.spec {
		v := vs[s.Name]
		if len(v) == 0 && s.Required {
			panic(fmt.Errorf("missing %q", s.Name))
		}
		switch s.Type {
		case "string":
			if len(v) == 0 {
				p.opts[s.Name] = []string{s.defval.(string)}
			} else {
				p.opts[s.Name] = v
			}
		case "int":
			var is []int64
			for _, a := range v {
				i, err := strconv.Atoi(a)
				if err != nil {
					panic(fmt.Errorf("%q is not an integer (arg:%s)", a, s.Name))
				}
				is = append(is, int64(i))
			}
			if len(is) == 0 {
				is = []int64{s.defval.(int64)}
			}
			p.opts[s.Name] = is
		case "float":
			var fs []float64
			for _, a := range v {
				f, err := strconv.ParseFloat(a, 64)
				if err != nil {
					panic(fmt.Errorf("%q is not a float (arg:%s)", a, s.Name))
				}
				fs = append(fs, f)
			}
			if len(fs) == 0 {
				fs = []float64{s.defval.(float64)}
			}
			p.opts[s.Name] = fs
		case "bool":
			var bs []bool
			for _, a := range v {
				b := true
				if a != "" {
					b, err = strconv.ParseBool(a)
					if err != nil {
						panic(fmt.Errorf("%q is not a bool (arg:%s)", a, s.Name))
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
}

func (p *Parser) Error() error {
	return p.err
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

func (p *Parser) Params() []Param {
	return p.spec
}

func NewParser(route string, spec []Param) (p *Parser, err error) {
	for i, s := range spec {
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
		spec[i].Type = t
		spec[i].defval = v
	}
	return &Parser{spec: spec, path: route}, nil
}
