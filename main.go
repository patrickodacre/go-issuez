package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"database/sql"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
)

var tpls *template.Template
var db *sql.DB

func main() {

	templateFuncs := template.FuncMap{}

	templates, err := findAndParseTemplates("templates", templateFuncs)

	handleFatalError(err, "Failed to parse templates.")

	tpls = templates

	for _, tpl := range tpls.Templates() {
		log.Println("tpl", tpl.Name())
	}

	db, e1 := sql.Open("postgres", "postgres://postgres:secret@172.17.0.2/postgres?sslmode=disable")

	handleFatalError(e1, "Failed to connect to db")

	defer db.Close()

	f, e2 := os.OpenFile("log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	handleFatalError(e2, "Failed to open log file")

	defer f.Close()

	logger := log.New(f, "", 1)

	router := httprouter.New()

	router.ServeFiles("/resources/*filepath", http.Dir("public/assets"))

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		tpls.ExecuteTemplate(w, "index.gohtml", nil)
	})

	// users
	userController := NewUsersController(db, logger)
	router.GET("/users", userController.index)
	router.GET("/users/:id", userController.show)
	router.POST("/users", userController.store)
	router.DELETE("/users/:id", userController.destroy)

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

			handleFatalError(err, "Failed to read template directory.")

			fileContents, err2 := ioutil.ReadFile(path)

			handleFatalError(err2, "Failed to read template file.")

			// use full file path to name the template
			name := path[templateNameBeginningIndex:]

			// New() adds the template by name
			t := root.New(name).Funcs(funcMap)

			// ensure each file can be parsed
			_, err2 = t.Parse(string(fileContents))
			handleFatalError(err2, "Failed to parse template file.")
		}

		return nil
	})

	return root, err
}

func handleError(err error, msg string) bool {

	if err != nil {
		log.Println(msg, err)

		return true
	}

	return false
}

func handleFatalError(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}
