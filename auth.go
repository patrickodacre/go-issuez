package main

import (
	"context"
	"database/sql"
	"github.com/sirupsen/logrus"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

func (s *authService) guard(next httprouter.Handle) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		s.log.Error("Auth Middleware Used")

		authUser, ok := s.getAuthUser(r)

		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)

			return
		}

		// add our auth user to our context so other handlers
		// have access to the user data
		ctx := context.WithValue(r.Context(), "user", authUser)

		next(w, r.WithContext(ctx), ps)
	}
}

func NewAuthService(db *sql.DB, logger *logrus.Logger, tpls *template.Template) *authService {
	return &authService{db, logger, tpls}
}

// Display a registration form.
func (s *authService) showRegistrationForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	_, ok := r.Context().Value("user").(user)

	if ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)

		return
	}

	pageData := page{Title: "Story Details", Data: nil}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/auth/registration-form.gohtml")
	err := view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *authService) registerUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

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
		pb := []byte(form_passwords[0])

		b, err := bcrypt.GenerateFromPassword(pb, bcrypt.DefaultCost)

		if err != nil {
			http.Error(w, "There was an error saving your password.", http.StatusUnprocessableEntity)
			return
		}

		password = string(b)
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

	// default to GUEST role
	stmt, err := s.db.Prepare(`
INSERT into goissuez.users (name, email, password, username, photo_url, role_id, created_at, updated_at, last_login )
VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id
`)

	if handleError(err, "Failed to prepare statement.") {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		id int64
	)

	err = stmt.QueryRow(name, email, password, username, photoPathToSave, GUEST).Scan(&id)

	if handleError(err, "Failed to query row.") {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.log.Info("Created user - ", id)

	// login
	err = s.authenticateUser(id, w)

	if err != nil {
		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	authUser := user{ID: id, Name: name, Email: email, Username: username, PhotoUrl: photoPathToSave.String}

	ctx := context.WithValue(r.Context(), "user", authUser)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)
}

func (s *authService) showLoginForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	_, ok := s.getAuthUser(r)

	// already logged in
	if ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)

		return
	}

	pageData := page{Title: "Login"}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/auth/loginform.gohtml")
	err := view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *authService) loginUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	r.ParseForm()

	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	authUser, err := getUserByUsername(s.db, username)

	if err != nil {
		s.log.Error("Error auth.loginuser.", err)

		http.Error(w, "Login failed.", http.StatusInternalServerError)
		return
	}

	if !s.verifyPassword(authUser, password) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = s.authenticateUser(authUser.ID, w)

	if err != nil {
		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	s.log.Error("Logged in user", authUser)

	ctx := context.WithValue(r.Context(), "user", authUser)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)
}

func (s *authService) verifyPassword(userData user, password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(userData.Password), []byte(password)); err != nil {
		return false
	}

	return true
}

// Update sessions table with a new session UUID and SetCookie
func (s *authService) authenticateUser(user_id int64, w http.ResponseWriter) error {
	// update the session:
	uuid, err := uuid.NewRandom()

	if err != nil {
		s.log.Error("Error ", err)
		return err
	}

	stmt, err := s.db.Prepare(`INSERT into goissuez.sessions (uuid, user_id, created_at) values ($1, $2, CURRENT_TIMESTAMP) RETURNING user_id`)

	if err != nil {
		s.log.Error("Error ", err)
		return err
	}

	defer stmt.Close()

	// clear out old sessions
	// it's easier to just record a new session and clear old ones
	// rather than update existing records with a new uuid
	defer func() {
		stmt, err := s.db.Prepare(`DELETE from goissuez.sessions WHERE user_id = $1 AND uuid != $2`)

		if err != nil {
			s.log.Error("Error s.db.Prepare() flushing sessions for user id: ", user_id, err)

			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(user_id, uuid.String())

		if err != nil {
			s.log.Error("Error stmt.Exec() flushing sessions for user id: ", user_id, err)
		}
	}()

	_, err = stmt.Exec(uuid.String(), user_id)

	if err != nil {
		s.log.Error("Error ", err)
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "goissuez",
		Value:  uuid.String(),
		MaxAge: (60 * 60 * 24),
		Path:   "/",
		// Secure: true,
		HttpOnly: true, // not available to JS
	})

	return nil
}

func (s *authService) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cookie, err := r.Cookie("goissuez")

	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)

		return
	}

	cookie.MaxAge = -1

	http.SetCookie(w, cookie)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *authService) demo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	pageData := page{Title: "Demo Login"}

	view := NewView(w, r)

	view.make("templates/auth/demo.gohtml")

	err := view.exec(mainLayout, pageData)

	if err != nil {
		http.Error(w, "Could not load page", http.StatusInternalServerError)
		return
	}

	view.send(http.StatusOK)
}

func (s *authService) getAuthUser(r *http.Request) (authUser user, ok bool) {
	// do we already have a user in the request context?
	authUser, ok = r.Context().Value("user").(user)

	if ok {
		return authUser, true
	}

	cookie, err := r.Cookie("goissuez")
	userData := user{Role: role{}}

	// no cookie == error
	if err != nil {
		return userData, false
	}

	stmt, err := s.db.Prepare(`
SELECT
u.id,
u.name,
u.username,
u.email,
u.created_at,
u.updated_at,
u.last_login,
u.role_id,
r.name as role_name,
r.description as role_description
FROM goissuez.users u
INNER JOIN goissuez.sessions s ON s.user_id = u.id
LEFT JOIN goissuez.roles r ON r.id = u.role_id
WHERE s.uuid = $1
LIMIT 1
`)

	if err != nil {
		s.log.Error("Error Prepare getAuthUser: ", err)
		return userData, false
	}

	defer stmt.Close()

	row := stmt.QueryRow(cookie.Value)

	roleID := sql.NullInt64{}
	role_name := sql.NullString{}
	role_description := sql.NullString{}

	if err := row.Scan(
		&userData.ID,
		&userData.Name,
		&userData.Username,
		&userData.Email,
		&userData.CreatedAt,
		&userData.UpdatedAt,
		&userData.LastLogin,
		&roleID,
		&role_name,
		&role_description,
	); err != nil {
		return userData, false
	}

	if roleID.Valid {
		userData.RoleID = roleID.Int64
		permissions, err := admin.getRolePermissions(userData.RoleID)

		if err != nil {
			s.log.Error("error getting user permissions", err)
			return userData, false
		}

		userData.Permissions = permissions
	}

	if role_name.Valid {
		userData.Role.Name = role_name.String
	}

	if role_description.Valid {
		userData.Role.Description = role_description.String
	}

	userData.IsAdmin = roleID.Valid && roleID.Int64 == ADMIN
	userData.CanAdmin = userData.Can([]string{"admin"})

	return userData, true
}
