package main

import (
	"html/template"
	"io"
	"log"
)

type page struct {
	Title string
	Data  interface{}
	Content interface{}
}

type viewService struct {
	w io.Writer
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

func (s *viewService) exec(layout string, pageData page) {
	s.t.ExecuteTemplate(s.w, layout, pageData)
}
