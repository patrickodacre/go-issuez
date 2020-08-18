package main

import (
	"database/sql"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
)

var tpls *template.Template
var db *sql.DB
var admin *adminService
var auth *authService
var users *userService
var projects *projectService
var features *featureService
var stories *storyService
var bugs *bugService
var log *logrus.Logger
var mainLayout string
var bufpool *bpool.BufferPool

const (
	ADMIN = 1
)

func init() {
	bufpool = bpool.NewBufferPool(64)
}

func main() {

	mainLayout = "dashboard_layout"

	f, e2 := os.OpenFile("log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	handleFatalError(e2, "Failed to open log file")

	defer f.Close()

	log = logrus.New()

	log.Formatter = &logrus.JSONFormatter{}

	log.Out = f

	templateFuncs := template.FuncMap{}

	tpls, err := findAndParseTemplates("templates", templateFuncs)

	handleFatalError(err, "Failed to parse templates.")

	db, e1 := sql.Open("postgres", "postgres://postgres:secret@172.17.0.2/postgres?sslmode=disable")

	handleFatalError(e1, "Failed to connect to db")

	defer db.Close()

	router := httprouter.New()

	router.ServeFiles("/resources/*filepath", http.Dir("public/assets"))

	admin = NewAdminService(db, log, tpls)
	users = NewUserService(db, log, tpls)
	auth = NewAuthService(db, log, tpls)
	projects = NewProjectService(db, log, tpls)
	features = NewFeatureService(db, log, tpls)
	stories = NewStoryService(db, log, tpls)
	bugs = NewBugService(db, log, tpls)

	router.GET("/", auth.demo)
	router.GET("/demo/:role", admin.demo)
	router.GET("/admin", auth.guard(admin.index))
	router.GET("/admin/users", auth.guard(admin.users))
	router.POST("/admin/setUserRole", auth.guard(admin.setUserRole))
	router.GET("/admin/roles", auth.guard(admin.roles))
	router.GET("/admin/roles/new", auth.guard(admin.createRole))
	router.POST("/roles/:role_id/update", auth.guard(admin.updateRole))
	router.GET("/roles/:role_id/edit", auth.guard(admin.editRole))
	router.POST("/roles", auth.guard(admin.storeRole))
	router.GET("/roles/:role_id", auth.guard(admin.role))
	router.DELETE("/roles/:role_id", auth.guard(admin.destroyRole))
	router.POST("/admin/permissions/:role_id", auth.guard(admin.savePermissions))

	router.GET("/users/:user_id", users.show)

	router.GET("/users/:user_id/projects", auth.guard(users.projects))
	router.GET("/users/:user_id/features", auth.guard(users.features))
	router.GET("/users/:user_id/stories", auth.guard(users.stories))
	router.GET("/users/:user_id/bugs", auth.guard(users.bugs))

	router.POST("/users", users.store)
	router.DELETE("/users/:user_id", users.destroy)

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
	router.GET("/features", auth.guard(features.all))
	router.GET("/projects/:project_id/features", auth.guard(features.index))
	router.POST("/projects/:project_id/features", auth.guard(features.store))

	router.GET("/projects/:project_id/features/new", auth.guard(features.create))
	router.GET("/features/:feature_id/edit", auth.guard(features.edit))
	router.POST("/features/:feature_id/update", auth.guard(features.update))
	router.GET("/features/:feature_id", auth.guard(features.show))
	router.DELETE("/features/:feature_id", auth.guard(features.destroy))

	// Stories
	router.GET("/stories", auth.guard(stories.all))
	router.GET("/features/:feature_id/stories", auth.guard(stories.index))
	router.POST("/features/:feature_id/stories", auth.guard(stories.store))

	router.GET("/features/:feature_id/stories/new", auth.guard(stories.create))
	router.GET("/stories/:story_id/edit", auth.guard(stories.edit))
	router.POST("/stories/:story_id/update", auth.guard(stories.update))
	router.GET("/stories/:story_id", auth.guard(stories.show))
	router.DELETE("/stories/:story_id", auth.guard(stories.destroy))

	// Bugs
	router.GET("/bugs", auth.guard(bugs.all))
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
		log.Fatal(msg, err)

		return true
	}

	return false
}

func handleFatalError(err error, msg string) {
	if err != nil {
		log.Fatal(msg, err)
	}
}

func deletedEntityNotice(message string, w http.ResponseWriter, r *http.Request, log *logrus.Logger) {
	pageData := page{Title: "Deleted Entity", Data: message}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/admin/deleted_entity.gohtml")
	err := view.exec(mainLayout, pageData)

	if err != nil {
		log.Error("Error: deletedEntityNotice", err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
	return
}
