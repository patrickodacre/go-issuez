package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
)

type page struct {
	Title string
	Data  interface{}
	Content interface{}
	AuthUser user
	IsLoggedIn bool
}

type viewService struct {
	w io.Writer
	r *http.Request
	log *log.Logger
	t *template.Template
}

func (s *viewService) make(filesnames ...string) {

	// get ALL available layouts
	tpls := template.Must(template.New("").ParseGlob("templates/layouts/*.gohtml"))

	// now parse the specific content files we want
	_, err := tpls.ParseFiles(filesnames...)

	if err != nil {
		s.log.Println("Error parsing template files", err)
	}

	s.t = tpls
}

func (s *viewService) exec(layout string, data interface{}) {

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

	s.t.ExecuteTemplate(s.w, layout, pageData)
}
