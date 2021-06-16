package render

import (
	"bytes"
	"encoding/json"
	"net/http"
	"text/template"

	"github.com/n-creativesystem/go-fwncs/bytesconv"
	"github.com/n-creativesystem/go-fwncs/constant"
)

type JSON struct {
	Data interface{}
}

func (r JSON) Render(w http.ResponseWriter) error {
	return WriteJson(w, r.Data)
}

func (r JSON) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.JSON)
}

func WriteJson(w http.ResponseWriter, data interface{}) error {
	writeContentType(w, constant.JSON)
	return json.NewEncoder(w).Encode(data)
}

type AsciiJSON struct {
	Data interface{}
}

func (r AsciiJSON) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	return json.NewEncoder(w).Encode(r.Data)
}

func (r AsciiJSON) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.JSONAscii)
}

type IndentJSON struct {
	Data   interface{}
	Indent string
}

func (r IndentJSON) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	e := json.NewEncoder(w)
	if r.Indent == "" {
		r.Indent = "    "
	}
	e.SetIndent("", r.Indent)
	return e.Encode(r.Data)
}

func (r IndentJSON) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.JSON)
}

type JSONP struct {
	Data     interface{}
	Callback string
}

func (r JSONP) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	buf := &bytes.Buffer{}
	if r.Callback == "" {
		return json.NewEncoder(w).Encode(r.Data)
	}
	err := json.NewEncoder(buf).Encode(r.Data)
	if err != nil {
		return err
	}
	callback := template.JSEscapeString(r.Callback)
	_, err = w.Write(bytesconv.StringToBytes(callback))
	if err != nil {
		return err
	}
	_, err = w.Write(bytesconv.StringToBytes("("))
	if err != nil {
		return err
	}
	_, err = w.Write(buf.Bytes())
	if err != nil {
		return err
	}
	_, err = w.Write(bytesconv.StringToBytes((");")))
	if err != nil {
		return err
	}
	return nil
}

func (r JSONP) WriteContentType(w http.ResponseWriter) {
	writeContentType(w, constant.JSONP)
}
