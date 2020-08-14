package main

import (
	"context"
	"database/sql"
	"errors"
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

	stmt, err := s.db.Prepare(`
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

	if handleError(err, "Failed to query row.") {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	s.log.Info("Created user - ", id)

	// login
	authUser, err := s.authenticateUser(username, password, s.db, w)

	if err != nil {
		if err.Error() == "unauthorized" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	s.log.Info("Logged in user", authUser)

	ctx := context.WithValue(r.Context(), "user", authUser)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)

	// redirect to GET("/users/:id")
	// this redirect will not work if the status isn't 303
	// http.Redirect(w, r, "/users/"+string(id), http.StatusSeeOther)
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

	authUser, err := s.authenticateUser(username, password, s.db, w)

	if err != nil {
		if err.Error() == "unauthorized" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

		http.Error(w, "Cannot login.", http.StatusInternalServerError)

		return
	}

	s.log.Error("Logged in user", authUser)

	ctx := context.WithValue(r.Context(), "user", authUser)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)
}

// Update sessions table with a new session UUID and SetCookie
func (s *authService) authenticateUser(username string, password string, db *sql.DB, w http.ResponseWriter) (user, error) {

	stmt, err := db.Prepare(`SELECT id, name, email, username, password, photo_url FROM goissuez.users u WHERE u.username = $1 LIMIT 1`)

	if err != nil {
		s.log.Error("Error ", err)
		return user{}, err
	}

	defer stmt.Close()

	row := stmt.QueryRow(username)

	var authUser user
	var photo_url sql.NullString

	err = row.Scan(
		&authUser.ID,
		&authUser.Name,
		&authUser.Email,
		&authUser.Username,
		&authUser.Password,
		&photo_url,
	)

	if err != nil {
		s.log.Error("Error ", err)
		return user{}, errors.New("Failed to login.")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(authUser.Password), []byte(password)); err != nil {
		e := errors.New("unauthorized")
		return user{}, e
	}

	if photo_url.Valid {
		authUser.PhotoUrl = photo_url.String
	}

	// update the session:
	uuid, err := uuid.NewRandom()

	if err != nil {
		s.log.Error("Error ", err)
		return user{}, err
	}

	stmt, err = db.Prepare(`INSERT into goissuez.sessions (uuid, user_id, created_at) values ($1, $2, CURRENT_TIMESTAMP) RETURNING user_id`)

	if err != nil {
		s.log.Error("Error ", err)
		return user{}, err
	}

	defer stmt.Close()

	// clear out old sessions
	// it's easier to just record a new session and clear old ones
	// rather than update existing records with a new uuid
	defer func() {
		stmt, err := s.db.Prepare(`DELETE from goissuez.sessions WHERE user_id = $1 AND uuid != $2`)

		if err != nil {
			s.log.Error("Error s.db.Prepare() flushing sessions for user id: ", authUser.ID, err)

			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(authUser.ID, uuid.String())

		if err != nil {
			s.log.Error("Error stmt.Exec() flushing sessions for user id: ", authUser.ID, err)
		}
	}()

	_, err = stmt.Exec(uuid.String(), authUser.ID)

	if err != nil {
		s.log.Error("Error ", err)
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

	return authUser, nil
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

func (s *authService) getAuthUser(r *http.Request) (authUser user, ok bool) {
	cookie, err := r.Cookie("goissuez")
	userData := user{}

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
u.role_id
FROM goissuez.users u
INNER JOIN goissuez.sessions s ON s.user_id = u.id
WHERE s.uuid = $1
LIMIT 1
`)

	if err != nil {
		s.log.Error("Error Prepare getAuthUser: ", err)
		return userData, false
	}

	row := stmt.QueryRow(cookie.Value)

	roleID := sql.NullInt64{}
	// permissions := []string{}

	if err := row.Scan(
		&userData.ID,
		&userData.Name,
		&userData.Email,
		&userData.Username,
		&userData.CreatedAt,
		&userData.UpdatedAt,
		&userData.LastLogin,
		&roleID,
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

	userData.IsAdmin = roleID.Valid && roleID.Int64 == ADMIN

	return userData, true
}

func (s *authService) can(user user, capabilities []string) bool {
	for _, c := range capabilities {

		_, ok := user.Permissions[c]

		if !ok {
			return false
		}
	}

	return true
}
