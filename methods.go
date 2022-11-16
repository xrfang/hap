package hap

import "strings"

const (
	hmGET = 1 << iota
	hmPOST
	hmDELETE
	hmHEAD
	hmOPTIONS
	hmPUT
	hmPATCH
	hmCONNECT
	hmTRACE
)

type (
	HttpMethod  string
	HttpMethods []HttpMethod
)

func (hm HttpMethod) Value() uint16 {
	switch strings.ToUpper(string(hm)) {
	case "GET":
		return hmGET
	case "POST":
		return hmPOST
	case "DELETE":
		return hmDELETE
	case "HEAD":
		return hmHEAD
	case "OPTIONS":
		return hmOPTIONS
	case "PUT":
		return hmPUT
	case "PATCH":
		return hmPATCH
	case "CONNECT":
		return hmCONNECT
	case "TRACE":
		return hmTRACE
	}
	return 0
}

func (hm HttpMethod) String() string {
	return strings.ToUpper(string(hm))
}

func (hms HttpMethods) Value() uint16 {
	if len(hms) == 0 {
		return 0xFFFF
	}
	var mv uint16
	for _, m := range hms {
		if v := m.Value(); v == 0 {
			return 0
		} else {
			mv |= v
		}
	}
	return mv
}

func (hms HttpMethods) Contains(hm HttpMethod) bool {
	return hms.Value()&hm.Value() != 0
}

func (hms HttpMethods) String() string {
	var ms []string
	for _, m := range hms {
		ms = append(ms, string(m))
	}
	return strings.Join(ms, ",")
}
