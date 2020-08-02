package main

import (
	"database/sql"
	"html/template"
	"io/ioutil"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
)

type userService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type user struct {
	ID        int64
	Name      string
	Username  string
	PhotoUrl  string
	Email     string
	CreatedAt string
	UpdatedAt string
	LastLogin string
}

func NewUserService(db *sql.DB, logger *logrus.Logger, tpls *template.Template) *userService {
	return &userService{db, logger, tpls}
}

func (s *userService) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	s.log.Error("Listing users.")

	stmt, err := s.db.Prepare(`select id, name, email, username, photo_url from goissuez.users`)

	if handleError(err, "Error listing users.") {
		http.Error(w, err.Error(), 500)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if handleError(err, "Error listing users.") {
		http.Error(w, err.Error(), 500)

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
			photo_url  sql.NullString
			created_at string
			updated_at string
			last_login string
		)

		if err := rows.Scan(&id, &name, &email, &password, &username, &photo_url, &created_at, &updated_at, &last_login); err != nil {
			log.Fatal(err)
		}

		users = append(users, user{ID: id, Name: name, Email: email, PhotoUrl: photo_url.String})
	}

	err = s.tpls.ExecuteTemplate(w, "users/users.gohtml", struct{ Users []user }{users})

	handleError(err, "Failed to execute users.gohtml.")
}

func (s *userService) store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 1 mb
	const MAX_MEMORY = 1 * 1024 * 1024

	r.ParseMultipartForm(MAX_MEMORY)

	var name string
	var username string
	var email string
	var password string

	form_names := r.MultipartForm.Value["name"]
	form_usernames := r.MultipartForm.Value["username"]
	form_emails := r.MultipartForm.Value["email"]
	form_passwords := r.MultipartForm.Value["password"]

	// all of these fields are required
	if len(form_names) > 0 && form_names[0] != "" {
		name = form_names[0]
	} else {
		http.Error(w, "NAME is required.", http.StatusUnprocessableEntity)
		return
	}

	if len(form_usernames) > 0 && form_usernames[0] != "" {
		username = form_usernames[0]
	} else {
		http.Error(w, "USERNAME is required.", http.StatusUnprocessableEntity)
		return
	}

	if len(form_emails) > 0 && form_emails[0] != "" {
		email = form_emails[0]
	} else {
		http.Error(w, "EMAIL is required.", http.StatusUnprocessableEntity)
		return
	}

	if len(form_passwords) > 0 && form_passwords[0] != "" {
		password = form_passwords[0]
	} else {
		http.Error(w, "PASSWORD is required.", http.StatusUnprocessableEntity)
		return
	}

	// Profile photo is NOT required
	// must use sql.NullString to ensure we have NULL set in the DB
	// when we don't have a file.
	var photoPathToSave sql.NullString
	{
		// the profile_url is not required
		pic, pic_header, err := r.FormFile("pic")

		// there will be an error if no file is selected for upload
		if err == nil && pic_header.Filename != "" {

			bs, err := ioutil.ReadAll(pic)

			if err != nil {
				http.Error(w, err.Error(), 500)

				return
			}

			photoPathToSave.String = "img/users/" + pic_header.Filename
			photoPathToSave.Valid = true

			dstPath := filepath.Join("./public/assets/" + photoPathToSave.String)

			dst, err := os.Create(dstPath)

			if err != nil {
				http.Error(w, err.Error(), 500)

				return
			}

			defer dst.Close()

			// save the actual file contents
			_, err = dst.Write(bs)

			if err != nil {
				http.Error(w, err.Error(), 500)

				return
			}
		}
	}

	s.log.Error("Creating user: ", name, email, password, username, photoPathToSave)

	stmt, err := s.db.Prepare(`
insert into goissuez.users (name, email, password, username, photo_url, created_at, updated_at, last_login )
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id
`)

	if handleError(err, "Failed to prepare statement.") {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		id int64
	)

	err = stmt.QueryRow(name, email, password, username, photoPathToSave).Scan(&id)

	if handleError(err, "Failed to query  row.") {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.log.Error("Created user - ", string(id))

	// redirect to GET("/users/:id")
	// this redirect will not work if the status isn't 303
	http.Redirect(w, r, "/users/"+string(id), http.StatusSeeOther)
}

func (s *userService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	user_id := ps.ByName("id")

	// TODO: get user by id:
	stmt, e1 := s.db.Prepare(`select * from goissuez.users where id = $1 limit 1`)

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
		photo_url  string
		created_at string
		updated_at string
		last_login string
	)

	e2 := row.Scan(&id, &name, &email, &password, &username, &photo_url, &created_at, &updated_at, &last_login)

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

	e3 := s.tpls.ExecuteTemplate(w, "users/user.gohtml", struct{ User user }{userProfile})

	handleError(e3, "Failed to execture user.gohtml")
}

func (s *userService) dashboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, ok := r.Context().Value("user").(user)

	s.log.Error("Dashboard -- ", authUser, ok)

	if !ok {
		_, ok = auth.getAuthUser(r)

		// no auth user, must login:
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}
	}

	s.tpls.ExecuteTemplate(w, "users/dashboard.gohtml", struct{ User user }{authUser})
}

func (s *userService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user_id := ps.ByName("id")

	stmt, e1 := s.db.Prepare(`DELETE from goissuez.users WHERE id = $1`)

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
