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

type user struct {
	ID        int64
	Name      string
	Username  string
	Email     string
	CreatedAt string
	UpdatedAt string
	LastLogin string
}

func main() {

	templateFuncs := template.FuncMap{}

	tpls, _ := findAndParseTemplates("templates", templateFuncs)

	for _, tpl := range tpls.Templates() {

		log.Println("tpl", tpl.Name())
	}

	db, err := sql.Open("postgres", "postgres://postgres:secret@172.17.0.2/postgres?sslmode=disable")

	if err != nil {
		log.Fatalln("Could not connect to db", err)
	}

	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		tpls.ExecuteTemplate(w, "index.gohtml", nil)
	})

	router.DELETE("/users/:id", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		user_id := ps.ByName("id")

		stmt, e1 := db.Prepare(`DELETE from goissuez.users WHERE id = $1`)

		if e1 != nil {
			log.Println("Error deleting user ", user_id, e1)
		}

		defer stmt.Close()

		_, e2 := stmt.Exec(user_id)

		if e2 != nil {
			log.Println("Failed to delete user.", e2)
		}

		// rowsAffected, e3 := result.RowsAffected()

		// if e3 != nil || rowsAffected == 0 {
			// log.Println("Failed to delete user.", e2)
		// }

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("Success"))
	})

	router.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		// /users/new - register a new user
		user_id := ps.ByName("id")

		if user_id == "new" {
			tpls.ExecuteTemplate(w, "users/new.gohtml", nil)

			return
		}

		// otherwise, the intent is to visit a user profile page

		// TODO: get user by id:
		stmt, errPrepare := db.Prepare(`select * from goissuez.users where id = $1 limit 1`)

		if errPrepare != nil {
			log.Println("errPrepare", errPrepare)
		}

		defer stmt.Close()

		row := stmt.QueryRow(user_id)

		var (
			id         int64
			name       string
			email      string
			password   string
			username   string
			created_at string
			updated_at string
			last_login string
		)

		if scanErr := row.Scan(&id, &name, &email, &password, &username, &created_at, &updated_at, &last_login); scanErr != nil {
			log.Println("Get user error", scanErr)
		}

		userProfile := user{
			ID:        id,
			Name:      name,
			Username:  username,
			Email:     email,
			CreatedAt: created_at,
			UpdatedAt: updated_at,
			LastLogin: last_login,
		}

		type Data struct {
			User user
		}

		tpls.ExecuteTemplate(w, "users/user.gohtml", Data{User: userProfile})
	})

	router.GET("/users", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		stmt, errPrepare := db.Prepare(`select * from goissuez.users`)

		if errPrepare != nil {
			log.Println("errPrepare", errPrepare)
		}

		defer stmt.Close()

		rows, errQuery := stmt.Query()

		if errQuery != nil {
			log.Println("errQuery", errQuery)
		}

		users := []user{}

		for rows.Next() {
			var (
				id         int64
				name       string
				email      string
				password   string
				username   string
				created_at string
				updated_at string
				last_login string
			)

			if err := rows.Scan(&id, &name, &email, &password, &username, &created_at, &updated_at, &last_login); err != nil {
				log.Fatal(err)
			}

			users = append(users, user{ID: id, Name: name, Email: email})
		}

		tplerr := tpls.ExecuteTemplate(w, "users/users.gohtml", struct{ Users []user }{users})

		if tplerr != nil {
			log.Println("Error looking at users", tplerr)
		}
	})

	router.POST("/users", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		r.ParseForm()

		name := r.PostForm.Get("name")
		email := r.PostForm.Get("email")
		password := r.PostForm.Get("password")
		username := r.PostForm.Get("username")

		stmt, stmtErr := db.Prepare(`
insert into goissuez.users (name, email, password, username, created_at, updated_at, last_login )
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING *;
`)

		if stmtErr != nil {
			log.Println("Statement Error", stmtErr.Error())
		}

		defer stmt.Close()

		var (
			id         int64
			_name      string
			_email     string
			_password  string
			_username  string
			created_at string
			updated_at string
			last_login string
		)

		scanErr := stmt.QueryRow(name, email, password, username).Scan(&id, &_name, &_email, &_password, &_username, &created_at, &updated_at, &last_login)

		if scanErr != nil {
			log.Println("Exec Statement Error", scanErr)
		}

		log.Println("id", id)

		// redirect to GET("/users/:id")
		// this redirect will not work if the status isn't 303
		http.Redirect(w, r, "/users/"+string(id), http.StatusSeeOther)
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
