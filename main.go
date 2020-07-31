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
var auth *authService
var users *userService
var projects *projectService
var features *featureService
var stories *storyService
var bugs *bugService

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

	users = NewUserService(db, logger, tpls)
	auth = NewAuthService(db, logger, tpls)
	projects = NewProjectService(db, logger, tpls)
	features = NewFeatureService(db, logger, tpls)
	stories = NewStoryService(db, logger, tpls)
	bugs = NewBugService(db, logger, tpls)

	router.GET("/users", users.index)
	router.GET("/users/:id", users.show)
	router.POST("/users", users.store)
	router.DELETE("/users/:id", users.destroy)

	router.GET("/dashboard", auth.guard(users.dashboard))

	router.GET("/register", auth.showRegistrationForm)
	router.POST("/register-user", auth.registerUser)
	router.GET("/login", auth.showLoginForm)
	router.POST("/login-user", auth.loginUser)
	router.GET("/logout", auth.logout)

	router.GET("/projects/:project_id/edit", auth.guard(projects.edit))
	router.POST("/projects/:project_id/update", auth.guard(projects.update))

	// projects.show will redirect to create form if :project_id == "new"
	router.GET("/projects/:project_id", auth.guard(projects.show))

	router.GET("/projects", auth.guard(projects.index))
	router.POST("/projects", auth.guard(projects.store))
	router.DELETE("/projects/:project_id", auth.guard(projects.destroy))

	// features are the parent issue type that will have child stories and bugs
	router.GET("/projects/:project_id/features", auth.guard(features.index))
	router.POST("/projects/:project_id/features", auth.guard(features.store))

	router.GET("/projects/:project_id/features/new", auth.guard(features.create))
	router.GET("/features/:feature_id/edit", auth.guard(features.edit))
	router.POST("/features/:feature_id/update", auth.guard(features.update))
	router.GET("/features/:feature_id", auth.guard(features.show))
	router.DELETE("/features/:feature_id", auth.guard(features.destroy))

	// Stories
	router.GET("/features/:feature_id/stories", auth.guard(stories.index))
	router.POST("/features/:feature_id/stories", auth.guard(stories.store))

	router.GET("/features/:feature_id/stories/new", auth.guard(stories.create))
	router.GET("/stories/:story_id/edit", auth.guard(stories.edit))
	router.POST("/stories/:story_id/update", auth.guard(stories.update))
	router.GET("/stories/:story_id", auth.guard(stories.show))
	router.DELETE("/stories/:story_id", auth.guard(stories.destroy))

	// Bugs
	router.GET("/features/:feature_id/bugs", auth.guard(bugs.index))
	router.POST("/features/:feature_id/bugs", auth.guard(bugs.store))

	router.GET("/features/:feature_id/bugs/new", auth.guard(bugs.create))
	router.GET("/bugs/:bug_id/edit", auth.guard(bugs.edit))
	router.POST("/bugs/:bug_id/update", auth.guard(bugs.update))
	router.GET("/bugs/:bug_id", auth.guard(bugs.show))
	router.DELETE("/bugs/:bug_id", auth.guard(bugs.destroy))

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
