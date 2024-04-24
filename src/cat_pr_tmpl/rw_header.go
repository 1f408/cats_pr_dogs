package main

import (
	"net/http"
	"net/textproto"
	"sync"
)

type RewriteHeaderHandler struct {
	hdl http.Handler
	rw  map[string]string
	mtx sync.Mutex
}

func (ra *RewriteHeaderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ra.Rewrite(r)
	ra.hdl.ServeHTTP(w, r)
}

func NewRewriteHeaderHandler(handle http.Handler) *RewriteHeaderHandler {
	return &RewriteHeaderHandler{hdl: handle, rw: map[string]string{}}
}

func (ra *RewriteHeaderHandler) SetRewrite(key string, val string) {
	ra.mtx.Lock()
	defer ra.mtx.Unlock()

	key = textproto.CanonicalMIMEHeaderKey(key)
	ra.rw[key] = val
}

func (ra *RewriteHeaderHandler) DelRewrite(key string) {
	ra.mtx.Lock()
	defer ra.mtx.Unlock()

	key = textproto.CanonicalMIMEHeaderKey(key)
	delete(ra.rw, key)
}

func (ra *RewriteHeaderHandler) Rewrite(r *http.Request) {
	ra.mtx.Lock()
	defer ra.mtx.Unlock()

	for k, v := range ra.rw {
		if v != "" {
			r.Header.Set(k, v)
		} else {
			r.Header.Del(k)
		}
	}
}
