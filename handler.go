package fwncs

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent/v3/integrations/nrhttprouter"
	"go.elastic.co/apm/module/apmhttprouter"
)

type IHandler interface {
	Handle(method, path string, handle httprouter.Handle)
	Handler(method, path string, handler http.Handler)
	HandlerFunc(method, path string, handler http.HandlerFunc)
	ServeHTTP(w http.ResponseWriter, req *http.Request)
	ServeFiles(path string, root http.FileSystem)
}

var _ IHandler = &apmhttprouter.Router{}
var _ IHandler = &nrhttprouter.Router{}
