package hap

import (
	"bytes"
	"encoding/json"
)

type Error struct {
	errs []error
	help string
	path string
	args []map[string]interface{}
}

func (e Error) Error() string {
	var bs bytes.Buffer
	je := json.NewEncoder(&bs)
	je.SetIndent("", "    ")
	je.SetEscapeHTML(false)
	m := map[string]interface{}{"for": e.help, "uri": e.path, "arg": e.args}
	if len(e.errs) > 0 {
		var errs []string
		for _, s := range e.errs {
			errs = append(errs, s.Error())
		}
		m["err"] = errs
	}
	je.Encode(m)
	return bs.String()
}
