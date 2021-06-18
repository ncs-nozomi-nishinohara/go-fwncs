package binding

import "net/http"

type Binding interface {
	Name() string
	Bind(r *http.Request, obj interface{}) error
}

type BindingBody interface {
	Binding
	BindBody(buf []byte, obj interface{}) error
}

type StructValidator interface {
	ValidateStruct(interface{}) error
	Engine() interface{}
}
