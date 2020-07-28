package main

import (
	"context"
	"database/sql"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

type authController struct {
	db  *sql.DB
	log *log.Logger
}

func NewAuthController(db *sql.DB, logger *log.Logger) *authController {
	return &authController{db, logger}
}

// Display a registration form.
func (c *authController) showRegistrationForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tpls.ExecuteTemplate(w, "users/new.gohtml", nil)
}

func (c *authController) registerUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

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

	c.log.Println("Creating user: ", name, email, password, username, photoPathToSave)

	stmt, err := c.db.Prepare(`
INSERT into goissuez.users (name, email, password, username, photo_url, created_at, updated_at, last_login )
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

	c.log.Println("Created user - ", id)

	// login
	userData, err := c.login(c.db, w, username, password)

	if err != nil {
		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	c.log.Println("Logged in user", userData)

	ctx := context.WithValue(r.Context(), "user", userData)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)

	// redirect to GET("/users/:id")
	// this redirect will not work if the status isn't 303
	// http.Redirect(w, r, "/users/"+string(id), http.StatusSeeOther)
}

func (c *authController) showLoginForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authUser := c.getAuthUser(r)

	// already logged in
	if authUser != (user{}) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)

		return
	}

	c.log.Println("Showing login form.")
	tpls.ExecuteTemplate(w, "users/loginform.gohtml", nil)
}

func (c *authController) loginUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	r.ParseForm()

	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	authUser, err := c.login(c.db, w, username, password)

	if err != nil {
		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	c.log.Println("Logged in user", authUser)

	ctx := context.WithValue(r.Context(), "user", authUser)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)
}

func (c *authController) dashboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	var ok bool
	var authUser user

	authUser, ok = r.Context().Value("user").(user)

	c.log.Println("Dashboard -- ", authUser, ok)

	if !ok {
		authUser = c.getAuthUser(r)

		// no auth user, must login:
		if authUser == (user{}) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}
	}

	tpls.ExecuteTemplate(w, "users/dashboard.gohtml", struct{ User user }{authUser})
}

// Update sessions table with a new session UUID and SetCookie
func (c *authController) login(db *sql.DB, w http.ResponseWriter, username string, password string) (user, error) {

	stmt, err := db.Prepare(`SELECT id, name, email, username, password, photo_url FROM goissuez.users u WHERE u.username = $1 LIMIT 1`)

	if err != nil {
		c.log.Println("Error ", err)
		return user{}, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(username)

	if err != nil {
		c.log.Println("Error ", err)
		return user{}, err
	}

	var userData user

	for rows.Next() {
		var (
			id        int64
			name      string
			email     string
			username  string
			password  string
			photo_url sql.NullString
			// created_at string
			// updated_at string
			// last_login string
		)

		if err := rows.Scan(&id, &name, &email, &username, &password, &photo_url); err != nil {
			c.log.Println("Error ", err)
			return user{}, err
		}

		userData = user{
			ID:       id,
			Name:     name,
			Email:    email,
			Username: username,
			PhotoUrl: photo_url.String,
			Password: password,
		}
	}

	if userData == (user{}) {
		c.log.Println("Error ", err)
		return user{}, errors.New("Failed to login.")
	}

	// TODO: compare password

	// update the session:
	uuid, err := uuid.NewRandom()

	if err != nil {
		c.log.Println("Error ", err)
		return user{}, err
	}

	stmt, err = db.Prepare(`INSERT into goissuez.sessions (uuid, user_id, created_at) values ($1, $2, CURRENT_TIMESTAMP) RETURNING user_id`)

	if err != nil {
		c.log.Println("Error ", err)
		return user{}, err
	}

	defer stmt.Close()

	// clear out old sessions
	// it's easier to just record a new session and clear old ones
	// rather than update existing records with a new uuid
	defer c.flushSessions(uuid.String(), userData.ID)

	_, err = stmt.Exec(uuid.String(), userData.ID)

	if err != nil {
		c.log.Println("Error ", err)
		return user{}, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "goissuez",
		Value:  uuid.String(),
		MaxAge: (60 * 60 * 24),
		Path:   "/",
		// Secure: true,
		HttpOnly: true, // not available to JS
	})

	return userData, nil
}

func (c *authController) flushSessions(uuid string, user_id int64) {

	stmt, err := c.db.Prepare(`DELETE from goissuez.sessions WHERE user_id = $1 AND uuid != $2`)

	if err != nil {
		c.log.Println("Error flushing sessions for user_id: ", user_id, err)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(user_id, uuid)

	if err != nil {
		c.log.Println("Error flushing sessions for user_id: ", user_id, err)

		return
	}
}

func (c *authController) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cookie, err := r.Cookie("goissuez")

	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return
	}

	cookie.MaxAge = -1

	http.SetCookie(w, cookie)

	w.Write([]byte("Logged Out"))
}

func (c *authController) getAuthUser(r *http.Request) user {
	cookie, err := r.Cookie("goissuez")

	if err != nil {
		return user{} // == nil
	}

	sql := `
SELECT u.id, u.name, u.username, u.email  from goissuez.users u
INNER JOIN goissuez.sessions s ON s.user_id = u.id
WHERE s.uuid = $1 LIMIT 1
`
	stmt, err := c.db.Prepare(sql)

	if err != nil {
		return user{}
	}

	rows, err := stmt.Query(cookie.Value)

	if err != nil {
		return user{}
	}

	var (
		id       int64
		name     string
		email    string
		username string
	)

	for rows.Next() {

		if err := rows.Scan(&id, &name, &email, &username); err != nil {
			return user{}
		}

	}

	return user{
		ID:       id,
		Name:     name,
		Email:    email,
		Username: username,
	}
}
