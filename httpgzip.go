package hap

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var (
	compLevel int = gzip.BestSpeed
	gzPool        = sync.Pool{
		New: func() interface{} {
			w := gzip.NewWriter(io.Discard)
			gzip.NewWriterLevel(w, compLevel)
			return w
		},
	}
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func EnableGZIP(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, func()) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || r.Method != http.MethodGet {
		return w, nil
	}
	gz := gzPool.Get().(*gzip.Writer)
	defer gzPool.Put(gz)
	gz.Reset(w)
	finalizer := func() {
		assert(gz.Flush())
		assert(gz.Close())
	}
	w.Header().Set("Content-Encoding", "gzip")
	return &gzipResponseWriter{ResponseWriter: w, Writer: gz}, finalizer
}

func SetGZLevel(level int) {
	compLevel = level
}
