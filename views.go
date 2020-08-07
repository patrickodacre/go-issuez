package main

import (
	"html/template"
	"net/http"
	"bytes"
)

type page struct {
	Title      string
	Data       interface{}
	Content    interface{}
	AuthUser   user
	IsLoggedIn bool
	Funcs      map[string]interface{}
}

type viewService struct {
	w http.ResponseWriter
	r *http.Request
	t *template.Template
	b *bytes.Buffer
}

func (s *viewService) make(filesnames ...string) {

	// get ALL available layouts
	tpls := template.Must(template.New("").ParseGlob("templates/layouts/*.gohtml"))

	// now parse the specific content files we want
	tpls = template.Must(tpls.ParseFiles(filesnames...))

	s.t = tpls
}

func (s *viewService) exec(layout string, data interface{}) error {

	var pageData page

	if data == nil {
		pageData = page{}
	} else {
		pageData = data.(page)
	}

	authUser, ok := auth.getAuthUser(s.r)

	if ok {
		pageData.AuthUser = authUser
		pageData.IsLoggedIn = true
	} else {
		pageData.AuthUser = user{}
		pageData.IsLoggedIn = false
	}

	s.b = bufpool.Get()

	return s.t.ExecuteTemplate(s.b, layout, pageData)
}

func (s *viewService) send(status int) {
	s.w.WriteHeader(status)
	s.b.WriteTo(s.w)

	defer bufpool.Put(s.b)
}
