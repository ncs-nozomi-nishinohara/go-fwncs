package render

import (
	"fmt"
	"net/http"
)

type Redirect struct {
	Status   int
	Request  *http.Request
	Location string
}

func (r Redirect) Render(w http.ResponseWriter) error {
	if (r.Status < http.StatusMultipleChoices || r.Status > http.StatusPermanentRedirect) && r.Status != http.StatusCreated {
		panic(fmt.Sprintf("Cannot redirect with status code %d", r.Status))
	}
	http.Redirect(w, r.Request, r.Location, r.Status)
	return nil
}

func (r Redirect) WriteContentType(w http.ResponseWriter) {}
