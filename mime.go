package fwncs

type mimeType string

func (m mimeType) Get() (key string, mime string) {
	key = "Content-Type"
	mime = string(m)
	return
}

// Content-Type MIME
const (
	MIMEJSON              mimeType = "application/json"
	MIMEHTML              mimeType = "text/html"
	MIMEXML               mimeType = "application/xml"
	MIMEXML2              mimeType = "text/xml"
	MIMEPlain             mimeType = "text/plain"
	MIMEPOSTForm          mimeType = "application/x-www-form-urlencoded"
	MIMEMultipartPOSTForm mimeType = "multipart/form-data"
	MIMEPROTOBUF          mimeType = "application/x-protobuf"
	MIMEMSGPACK           mimeType = "application/x-msgpack"
	MIMEMSGPACK2          mimeType = "application/msgpack"
	MIMEYAML              mimeType = "application/x-yaml"
)
