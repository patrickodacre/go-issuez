package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type userController struct {
	db  *sql.DB
	log *log.Logger
}

type user struct {
	ID        int64
	Name      string
	Username  string
	Email     string
	CreatedAt string
	UpdatedAt string
	LastLogin string
}

func NewUsersController(db *sql.DB, logger *log.Logger) *userController {
	return &userController{db, logger}
}

func (c *userController) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	c.log.Println("Listing users.")

	stmt, e1 := c.db.Prepare(`select * from goissuez.users`)

	if handleError(e1, "Error listing users.") {
		http.Error(w, e1.Error(), 500)

		return
	}

	defer stmt.Close()

	rows, e2 := stmt.Query()

	if handleError(e2, "Error listing users.") {
		http.Error(w, e2.Error(), 500)

		return
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

	e3 := tpls.ExecuteTemplate(w, "users/users.gohtml", struct{ Users []user }{users})

	handleError(e3, "Failed to execute users.gohtml.")
}

func (c *userController) store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	r.ParseForm()

	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	password := r.PostForm.Get("password")
	username := r.PostForm.Get("username")

	stmt, e1 := c.db.Prepare(`
insert into goissuez.users (name, email, password, username, created_at, updated_at, last_login )
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING *;
`)

	if handleError(e1, "Failed to prepare statement.") {
		http.Error(w, e1.Error(), 500)

		return
	}

	defer stmt.Close()

	var (
		id         int64
		_name      string
		_password  string
		_username  string
		_email     string
		created_at string
		updated_at string
		last_login string
	)

	e2 := stmt.QueryRow(name, email, password, username).Scan(&id, &_name, &_email, &_password, &_username, &created_at, &updated_at, &last_login)

	if handleError(e2, "Failed to query row.") {
		http.Error(w, e2.Error(), 500)

		return
	}

	// redirect to GET("/users/:id")
	// this redirect will not work if the status isn't 303
	http.Redirect(w, r, "/users/"+string(id), http.StatusSeeOther)
}

func (c *userController) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	// /users/new - register a new user
	user_id := ps.ByName("id")

	if user_id == "new" {
		tpls.ExecuteTemplate(w, "users/new.gohtml", nil)

		return
	}

	// otherwise, the intent is to visit a user profile page

	// TODO: get user by id:
	stmt, e1 := c.db.Prepare(`select * from goissuez.users where id = $1 limit 1`)

	if handleError(e1, "Failed to prepare show user.") {
		http.Error(w, e1.Error(), 500)

		return
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

	e2 := row.Scan(&id, &name, &email, &password, &username, &created_at, &updated_at, &last_login)

	if handleError(e2, "Failed to scan user row.") {
		http.Error(w, e2.Error(), 500)

		return
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

	e3 := tpls.ExecuteTemplate(w, "users/user.gohtml", struct{ User user }{userProfile})

	handleError(e3, "Failed to execture user.gohtml")
}

func (c *userController) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user_id := ps.ByName("id")

	stmt, e1 := c.db.Prepare(`DELETE from goissuez.users WHERE id = $1`)

	if handleError(e1, "Failed to delete user.") {
		http.Error(w, e1.Error(), 500)

		return
	}

	defer stmt.Close()

	_, e2 := stmt.Exec(user_id)

	if handleError(e2, "Failed to delete user.") {
		http.Error(w, e2.Error(), 500)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte("Success"))
}
