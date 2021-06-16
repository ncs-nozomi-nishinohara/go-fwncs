package render

import (
	"net/http"

	"github.com/n-creativesystem/go-fwncs/constant"
	"gopkg.in/yaml.v3"
)

type YAML struct {
	Data interface{}
}

func (r YAML) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	e := yaml.NewEncoder(w)
	defer e.Close()
	return e.Encode(r.Data)
}

func (r YAML) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.YAML)
}

type IndentYAML struct {
	IndentSpace int
	Data        interface{}
}

func (r IndentYAML) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	e := yaml.NewEncoder(w)
	if r.IndentSpace == 0 {
		r.IndentSpace = 2
	}
	e.SetIndent(r.IndentSpace)
	defer e.Close()
	return e.Encode(r.Data)
}

func (r IndentYAML) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.YAML)
}
