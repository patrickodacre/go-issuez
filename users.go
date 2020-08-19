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

	"github.com/julienschmidt/httprouter"
	"github.com/lib/pq"
)

type userService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type user struct {
	ID          int64
	Name        string
	Username    string
	Password    string
	PhotoUrl    string
	Email       string
	CreatedAt   string
	UpdatedAt   string
	LastLogin   string
	DeletedAt   string
	IsAdmin     bool
	CanAdmin    bool
	RoleID      int64
	Role        role
	Permissions map[string]capability
}

// can checks the authenticated user permissions.
// if the user isn't authenticated, then false will be returned
func (u *user) Can(capabilities []string) bool {

	if u.RoleID == 0 {
		return false
	}

	for _, c := range capabilities {

		_, ok := u.Permissions[c]

		if !ok {
			return false
		}
	}

	return true
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

		userData := user{}
		var photo_url sql.NullString

		if err := rows.Scan(
			&userData.ID,
			&userData.Name,
			&userData.Email,
			&userData.Username,
			&photo_url,
			&userData.CreatedAt,
			&userData.UpdatedAt,
			&userData.LastLogin,
		); err != nil {
			log.Fatal(err)
		}

		if photo_url.Valid {
			userData.PhotoUrl = photo_url.String
		}

		users = append(users, userData)
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

	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"read_users"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")

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

func (s *userService) projects(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"read_users", "read_projects_mine"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")

	// can ignore 'ok' b/c this route is behind auth.guard()
	// authUser, _ := r.Context().Value("user").(user)

	stmt, err := s.db.Prepare(`
SELECT id, name, description, user_id, created_at, updated_at
FROM goissuez.projects
WHERE user_id = $1
`)

	if err != nil {
		s.log.Error(err)

		http.Error(w, "Error listing user projects.", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(user_id)

	if err != nil {
		s.log.Error(err)

		http.Error(w, "Error listing user projects.", http.StatusInternalServerError)
		return
	}

	projects := []project{}

	for rows.Next() {
		projectData := project{}

		err := rows.Scan(
			&projectData.ID,
			&projectData.Name,
			&projectData.Description,
			&projectData.UserID,
			&projectData.CreatedAt,
			&projectData.UpdatedAt,
		)

		if err != nil {
			s.log.Error(err)

			http.Error(w, "Error listing user projects.", http.StatusInternalServerError)
			return
		}

		projects = append(projects, projectData)
	}

	pageData := page{Title: "User Projects", Data: projects}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/users/projects.gohtml")

	err = view.exec("dashboard_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

// List the features with which this user is involved.
// A user is involved in a feature if they are assigned
// to either a story or a bug associated with that feature.
func (s *userService) features(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	// a feature should be considered "MINE" if that user is involved with them somehow
	if !authUser.Can([]string{"read_features"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")
	feature_ids := []int64{}
	features := []feature{}
	userData, err := getUserByID(s.db, user_id)

	if err != nil {
		s.log.Error("Error users.features.query.user.", err)

		http.Error(w, "Error listing user features.", http.StatusInternalServerError)

		return
	}

	stmt, err := s.db.Prepare(`
SELECT
b.feature_id as bug_feature_id,
s.feature_id as story_feature_id
FROM
   (SELECT DISTINCT feature_id FROM goissuez.bugs b WHERE b.assignee_id = $1 ) b
   FULL OUTER JOIN
   	(SELECT DISTINCT feature_id FROM goissuez.stories s WHERE s.assignee_id = $1) s
   ON b.feature_id = s.feature_id
`)

	if err != nil {
		s.log.Error("Error users.features.prepare.feature_ids.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(user_id)

	if err != nil {
		s.log.Error("Error users.features.query.feature_ids.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		var bugFeatureID sql.NullInt64
		var storyFeatureID sql.NullInt64

		if err := rows.Scan(
			&bugFeatureID,
			&storyFeatureID,
		); err != nil {
			s.log.Error("Error users.features.scan.", err)

			http.Error(w, "Error listing features.", http.StatusInternalServerError)

			return
		}

		if bugFeatureID.Valid {
			feature_ids = append(feature_ids, bugFeatureID.Int64)
		} else if storyFeatureID.Valid {
			feature_ids = append(feature_ids, storyFeatureID.Int64)
		}

	}

	stmt, err = s.db.Prepare(`
SELECT
id,
name,
description,
created_at,
updated_at
FROM goissuez.features
WHERE id = ANY($1)
`)

	if err != nil {
		s.log.Error("Error users.features.prepare.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err = stmt.Query(pq.Array(feature_ids))

	if err != nil {
		s.log.Error("Error users.features.query.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {
		featureData := feature{}

		err := rows.Scan(
			&featureData.ID,
			&featureData.Name,
			&featureData.Description,
			&featureData.CreatedAt,
			&featureData.UpdatedAt,
		)

		if err != nil {
			s.log.Error("Error users.features.scan.", err)

			http.Error(w, "Error listing features.", http.StatusInternalServerError)

			return
		}

		features = append(features, featureData)
	}

	pageData := page{
		Title: userData.Name + " - Features",
		Data: struct {
			Features []feature
			Assignee user
		}{
			features,
			userData,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/users/features.gohtml")

	err = view.exec("dashboard_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *userService) stories(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"read_stories_mine"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")
	stories := []story{}
	userData, err := getUserByID(s.db, user_id)

	if err != nil {
		s.log.Error("Error users.stories.query.user.", err)

		http.Error(w, "Error listing user stories.", http.StatusInternalServerError)

		return
	}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
assignee_id,
created_at,
updated_at
FROM goissuez.stories
WHERE assignee_id = $1
ORDER BY updated_at
`)

	if err != nil {
		s.log.Error("Error users.stories.prepare.", err)

		http.Error(w, "Error listing user stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(user_id)

	if err != nil {
		s.log.Error("Error users.stories.query.", err)

		http.Error(w, "Error listing user stories.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		storyData := story{}

		err := rows.Scan(
			&storyData.ID,
			&storyData.Name,
			&storyData.AssigneeID,
			&storyData.CreatedAt,
			&storyData.UpdatedAt,
		)

		if err != nil {
			s.log.Error("Error users.stories.scan.", err)

			http.Error(w, "Error listing user stories.", http.StatusInternalServerError)

			return
		}

		stories = append(stories, storyData)
	}

	pageData := page{
		Title: userData.Name + " - Stories",
		Data: struct {
			Stories  []story
			Assignee user
		}{
			stories,
			userData,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/users/stories.gohtml")

	err = view.exec("dashboard_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *userService) bugs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"read_bugs_mine"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")
	bugs := []bug{}
	userData, err := getUserByID(s.db, user_id)

	if err != nil {
		s.log.Error("Error users.bugs.query.user.", err)

		http.Error(w, "Error listing user bugs.", http.StatusInternalServerError)

		return
	}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
assignee_id,
created_at,
updated_at
FROM goissuez.bugs
WHERE assignee_id = $1
ORDER BY updated_at
`)

	if err != nil {
		s.log.Error("Error users.bugs.prepare.", err)

		http.Error(w, "Error listing user bugs.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(user_id)

	if err != nil {
		s.log.Error("Error users.bugs.query.", err)

		http.Error(w, "Error listing user bugs.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		bugData := bug{}

		err := rows.Scan(
			&bugData.ID,
			&bugData.Name,
			&bugData.AssigneeID,
			&bugData.CreatedAt,
			&bugData.UpdatedAt,
		)

		if err != nil {
			s.log.Error("Error users.bugs.scan.", err)

			http.Error(w, "Error listing user bugs.", http.StatusInternalServerError)

			return
		}

		bugs = append(bugs, bugData)
	}

	pageData := page{
		Title: userData.Name + " - Bugs",
		Data: struct {
			Bugs     []bug
			Assignee user
		}{
			bugs,
			userData,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/users/bugs.gohtml")

	err = view.exec("dashboard_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *userService) dashboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	pageData := page{Title: "User Dashboard - " + authUser.Name, Data: nil}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/users/dashboard.gohtml")

	err := view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

// destory user SOFT DELETES a user by just adding a deleted_at field.
func (s *userService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"delete_users"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user_id := ps.ByName("user_id")

	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)

	if err != nil {
		s.log.Error("Error users.destroy.begintx.", err)

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	stmt, err := tx.Prepare(`
UPDATE goissuez.users
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1
`)

	if err != nil {
		s.log.Error("Error users.destroy.prepare.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(user_id)

	if err != nil {
		s.log.Error("Error users.destroy.exec.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	// update all stories
	stmt, err = tx.Prepare(`
UPDATE goissuez.stories
SET assignee_id = NULL
WHERE assignee_id = $1
`)

	if err != nil {
		s.log.Error("Error users.destroy.update.stories.prepare.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(user_id)

	if err != nil {
		s.log.Error("Error users.destroy.update.stories.exec.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	// update all bugs
	stmt, err = tx.Prepare(`
UPDATE goissuez.bugs
SET assignee_id = NULL
WHERE assignee_id = $1
`)

	if err != nil {
		s.log.Error("Error users.destroy.update.bugs.prepare.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(user_id)

	if err != nil {
		s.log.Error("Error users.destroy.update.bugs.exec.", err)

		tx.Rollback()

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	err = tx.Commit()

	if err != nil {
		s.log.Error("Error users.destroy.committx.", err)

		http.Error(w, "Cannot delete user.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func getUserByUsername(db *sql.DB, username string) (user, error) {
	userData := user{}

	stmt, err := db.Prepare(`SELECT id, name, email, username, password, photo_url FROM goissuez.users u WHERE u.username = $1 LIMIT 1`)

	if err != nil {
		return userData, err
	}

	defer stmt.Close()

	row := stmt.QueryRow(username)

	var photo_url sql.NullString

	err = row.Scan(
		&userData.ID,
		&userData.Name,
		&userData.Email,
		&userData.Username,
		&userData.Password,
		&photo_url,
	)

	if err != nil {
		return userData, err
	}

	if photo_url.Valid {
		userData.PhotoUrl = photo_url.String
	}

	return userData, nil
}

func getUserByID(db *sql.DB, user_id string) (user, error) {

	stmt, err := db.Prepare(`
SELECT
id,
name,
username,
photo_url,
email,
last_login
FROM goissuez.users
WHERE id = $1
LIMIT 1
`)

	if err != nil {
		return user{}, err
	}

	defer stmt.Close()

	row := stmt.QueryRow(user_id)

	userData := user{}

	var photo_url sql.NullString

	err = row.Scan(
		&userData.ID,
		&userData.Name,
		&userData.Username,
		&photo_url,
		&userData.Email,
		&userData.LastLogin,
	)

	if err != nil {
		return user{}, err
	}

	if photo_url.Valid {
		userData.PhotoUrl = photo_url.String
	}

	return userData, nil
}

func getUsers(db *sql.DB) ([]user, error) {
	users := []user{}

	stmt, err := db.Prepare(`
SELECT
id,
name,
email,
username,
photo_url,
created_at,
updated_at,
last_login
FROM goissuez.users
ORDER BY created_at
`)

	if err != nil {
		return []user{}, err
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		return []user{}, err
	}

	for rows.Next() {

		userData := user{}
		var photo_url sql.NullString

		if err := rows.Scan(
			&userData.ID,
			&userData.Name,
			&userData.Email,
			&userData.Username,
			&photo_url,
			&userData.CreatedAt,
			&userData.UpdatedAt,
			&userData.LastLogin,
		); err != nil {
			return []user{}, err
		}

		if photo_url.Valid {
			userData.PhotoUrl = photo_url.String
		}

		users = append(users, userData)
	}

	return users, nil
}
