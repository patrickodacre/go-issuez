package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
)

var tpls *template.Template

type user struct {
	Name string
	Email string
}

func main() {

	templateFuncs := template.FuncMap{}

	tpls, _ := findAndParseTemplates("templates", templateFuncs)

	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		tpls.ExecuteTemplate(w, "index.gohtml", nil)
	})

	router.GET("/users/:id", func (w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		// /users/new - register a new user
		if id := ps.ByName("id"); id == "new" {
			tpls.ExecuteTemplate(w, "users/new.gohtml", nil)

			return
		}

		// otherwise, the intent is to visit a user profile page

		// TODO: get user by id:
		userProfile := user{
			Name: "Danny",
			Email: "danny@testing.com",
		}

		type Data struct {
			User user
		}

		tpls.ExecuteTemplate(w, "users/user.gohtml", Data{User: userProfile})
	})

	router.POST("/users", func (w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		r.ParseForm()

		email := r.PostForm.Get("email")
		password := r.PostForm.Get("password")

		fakeId := 1

		// TODO: create new user in the db
		log.Println("email", email)
		log.Println("password", password)

		// redirect to GET("/users/:id")
		// this redirect will not work if the status isn't 303
		http.Redirect(w, r, "/users/" + string(fakeId), http.StatusSeeOther)
	})

	log.Fatal(http.ListenAndServe(":8080", router))
}

func findAndParseTemplates(rootDir string, funcMap template.FuncMap) (*template.Template, error) {
	// eg: "templates"
	cleanRoot := filepath.Clean(rootDir)
	// template names will begin with dir names after the root directory eg: "templates"
	templateNameBeginningIndex := len(cleanRoot) + 1
	root := template.New("")

	err := filepath.Walk(cleanRoot, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(path, ".gohtml") {
			if err != nil {
				return err
			}

			fileContents, err2 := ioutil.ReadFile(path)
			if err2 != nil {
				return err2
			}

			// use full file path to name the template
			name := path[templateNameBeginningIndex:]

			// New() adds the template by name
			t := root.New(name).Funcs(funcMap)

			// ensure each file can be parsed
			_, err2 = t.Parse(string(fileContents))
			if err2 != nil {
				return err2
			}
		}

		return nil
	})

	return root, err
}
