package hap

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
)

// parse query string in G-P-C order, i.e. GET (query string) has highest priority,
// POST (body) follows, and COOKIE has lowest priority
func args(r *http.Request) (url.Values, error) {
	vs := make(url.Values)
	for _, c := range r.Cookies() {
		vs[c.Name] = []string{c.Value}
	}
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
	for k, v := range r.URL.Query() {
		vs[k] = v
	}
	return vs, nil
}
