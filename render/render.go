package render

import (
	"fmt"
	"net/http"

	"github.com/n-creativesystem/go-fwncs/constant"
)

type Render interface {
	Render(w http.ResponseWriter) error
	WriteContentType(w http.ResponseWriter)
}

func writeContentType(w http.ResponseWriter, value fmt.Stringer) {
	w.Header().Set(constant.HeaderContentType, value.String())
}

var (
	_ Render = Text{}
	_ Render = JSON{}
	_ Render = AsciiJSON{}
	_ Render = JSONP{}
	_ Render = IndentJSON{}
	_ Render = Redirect{}
	_ Render = TemplateRender{}
)
