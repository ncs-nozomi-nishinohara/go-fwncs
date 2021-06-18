package binding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var EnableUseNumber = false

var EnableDecoderDisallowUnknownFields = false

type jsonBiding struct{}

var _ BindingBody = jsonBiding{}

func (j jsonBiding) Name() string {
	return "json"
}

func (j jsonBiding) Bind(r *http.Request, obj interface{}) error {
	if r == nil || r.Body == nil {
		return errors.New("invalid request")
	}
	return decodeJSON(r.Body, obj)
}

func (j jsonBiding) BindBody(buf []byte, obj interface{}) error {
	return decodeJSON(bytes.NewReader(buf), obj)
}

func decodeJSON(r io.Reader, obj interface{}) error {
	decoder := json.NewDecoder(r)
	if EnableUseNumber {
		decoder.UseNumber()
	}
	if EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(obj); err != nil {
		return err
	}

	return validate(obj)
}
