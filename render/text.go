package render

import (
	"fmt"
	"net/http"

	"github.com/n-creativesystem/go-fwncs/bytesconv"
	"github.com/n-creativesystem/go-fwncs/constant"
)

type Text struct {
	Format string
	Data   []interface{}
}

func (r Text) Render(w http.ResponseWriter) error {
	return WriteText(w, r.Format, r.Data...)
}

func (r Text) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.Plain)
}

func WriteText(w http.ResponseWriter, format string, v ...interface{}) error {
	writeContentType(w, constant.Plain)
	if len(v) > 0 {
		_, err := fmt.Fprintf(w, format, v...)
		return err
	}
	_, err := w.Write(bytesconv.StringToBytes(format))
	return err
}
