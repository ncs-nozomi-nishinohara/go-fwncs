package render

import (
	"html/template"
	"net/http"

	"github.com/n-creativesystem/go-fwncs/constant"
)

type TemplateRender struct {
	Template *template.Template
	Data     interface{}
}

func (r TemplateRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	return r.Template.Execute(w, r.Data)
}

func (r TemplateRender) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.HTML)
}
