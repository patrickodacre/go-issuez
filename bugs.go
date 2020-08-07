package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

type bugService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type bug struct {
	ID          int64
	Name        string
	Description string
	FeatureID   int64
	UserID      int64
	AssigneeID  int64
	CreatedAt   string
	UpdatedAt   string
	Creator     *user
	Assignee    *user
	Feature     *feature
	Project     *project
}

func NewBugService(db *sql.DB, log *logrus.Logger, tpls *template.Template) *bugService {
	return &bugService{db, log, tpls}
}

func (s *bugService) all(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bugs := []bug{}

	stmt, err := s.db.Prepare(`
SELECT
b.id,
b.name,
b.feature_id,
b.user_id,
b.assignee_id,
b.created_at,
b.updated_at,
f.name as feature_name
FROM goissuez.bugs b
JOIN goissuez.features f
ON f.id = b.feature_id
ORDER BY b.updated_at
`)

	if err != nil {
		s.log.Error("Error bugs.all.prepare.", err)
		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	rows, err := stmt.Query()

	if err != nil {
		s.log.Error("Error bugs.all.query.", err)
		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {
		bugData := bug{Feature: &feature{}}

		var assignee_id sql.NullInt64

		err := rows.Scan(
			&bugData.ID,
			&bugData.Name,
			&bugData.FeatureID,
			&bugData.UserID,
			&assignee_id,
			&bugData.CreatedAt,
			&bugData.UpdatedAt,
			&bugData.Feature.Name,
		)

		if err != nil {
			s.log.Error("Error bugs.all.scan.", err)
			http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

			return
		}

		if assignee_id.Valid {
			bugData.AssigneeID = assignee_id.Int64
		}

		bugs = append(bugs, bugData)
	}

	users, err := getUsers(s.db)

	usersByID := make(map[int64]*user)

	if err == nil {
		for i := 0; i < len(users); i++ {
			usersByID[users[i].ID] = &users[i]
		}
	}

	for i := 0; i < len(bugs); i++ {

		// make sure we're mutating the actual bug
		// in the slice
		bug := &bugs[i]

		creator, ok := usersByID[bug.UserID]

		if ok {
			bug.Creator = creator
		}

		assignee, ok := usersByID[bug.AssigneeID]

		if ok {
			bug.Assignee = assignee
		}
	}

	pageData := page{
		Title: "Bugs",
		Data: struct {
			Bugs []bug
		}{
			bugs,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/bugs/all.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *bugService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	featureData := feature{}

	parentFeatureID := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`SELECT id, name, description, project_id FROM goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Error("Error bugs.index.prepare.feature.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	err = stmt.QueryRow(parentFeatureID).Scan(&featureData.ID, &featureData.Name, &featureData.Description, &featureData.ProjectID)

	if err != nil {
		s.log.Error("Error bugs.index.scan.feature.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	query := `
SELECT
id,
name,
description,
feature_id,
user_id,
assignee_id,
created_at,
updated_at
FROM goissuez.bugs
WHERE feature_id = $1
ORDER BY created_at
`
	stmt, err = s.db.Prepare(query)

	if err != nil {

		s.log.Error("Error bugs.index.prepare.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(parentFeatureID)

	if err != nil {

		s.log.Error("Error bugs.index.query.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		bugData := &bug{}
		var assigneeID sql.NullInt64

		err := rows.Scan(
			&bugData.ID,
			&bugData.Name,
			&bugData.Description,
			&bugData.FeatureID,
			&bugData.UserID,
			&assigneeID,
			&bugData.CreatedAt,
			&bugData.UpdatedAt,
		)

		if err != nil {

			s.log.Error("Error bugs.index.scan.", err)

			http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

			return
		}

		if assigneeID.Valid {
			bugData.AssigneeID = assigneeID.Int64
		}

		featureData.Bugs = append(featureData.Bugs, *bugData)
	}

	pageData := page{Title: "Bugs", Data: featureData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/bugs/bugs.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *bugService) store(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	feature_id := ps.ByName("feature_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")
	assignee_id := r.PostForm.Get("assignee_id")

	query := `
INSERT INTO goissuez.bugs
(name, description, feature_id, user_id, assignee_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error bugs.store.prepare.", err)

		http.Error(w, "Error creating bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var assignee sql.NullInt64

	if assignee_id == "0" {
		assignee = sql.NullInt64{}
	} else {
		v, err := strconv.ParseInt(assignee_id, 10, 64)

		if err != nil {
			assignee = sql.NullInt64{}
		}

		assignee = sql.NullInt64{
			Int64: v,
			Valid: true,
		}
	}

	_, err = stmt.Exec(name, description, feature_id, authUser.ID, assignee)

	if err != nil {
		s.log.Error("Error bugs.store.exec.", err)

		http.Error(w, "Error saving bug.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/features/"+feature_id, http.StatusSeeOther)
}

func (s *bugService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	bug_id := ps.ByName("bug_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")
	assignee_id := r.PostForm.Get("assignee_id")

	query := `
UPDATE goissuez.bugs
SET name = $2, description = $3, assignee_id = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error bugs.update.prepare.", err)

		http.Error(w, "Error updating bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var assignee sql.NullInt64

	if assignee_id == "0" {
		assignee = sql.NullInt64{}
	} else {
		v, err := strconv.ParseInt(assignee_id, 10, 64)

		if err != nil {
			assignee = sql.NullInt64{}
		} else {
			assignee = sql.NullInt64{
				Int64: v,
				Valid: true,
			}
		}
	}

	_, err = stmt.Exec(bug_id, name, description, assignee)

	if err != nil {
		s.log.Error("Error bugs.update.exec.", err)

		http.Error(w, "Error updating bug.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/bugs/"+bug_id, http.StatusSeeOther)
}

func (s *bugService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bug_id := ps.ByName("bug_id")

	query := `
SELECT
id,
name,
description,
feature_id,
user_id,
assignee_id,
created_at,
updated_at

FROM goissuez.bugs
WHERE id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error bugs.edit.prepare.", err)

		http.Error(w, "Error editing bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(bug_id)

	bugData := bug{}
	var assigneeID sql.NullInt64

	err = row.Scan(
		&bugData.ID,
		&bugData.Name,
		&bugData.Description,
		&bugData.FeatureID,
		&bugData.UserID,
		&assigneeID,
		&bugData.CreatedAt,
		&bugData.UpdatedAt,
	)

	if err != nil {
		s.log.Error("Error bugs.edit.scan.", err)

		http.Error(w, "Error editing bug.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		bugData.AssigneeID = assigneeID.Int64
	}

	users, _ := getUsers(s.db)

	pageData := page{
		Title: "Edit Bug - " + bugData.Name,
		Data: struct {
			Bug   bug
			Users []user
		}{
			bugData,
			users,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/bugs/edit.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

// Show the new / create feature form.
func (s *bugService) create(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	parentFeatureID := ps.ByName("feature_id")

	query := `
SELECT
id,
name,
description,
user_id,
created_at,
updated_at
FROM goissuez.features
WHERE id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error bugs.create.prepare.", err)

		http.Error(w, "Error creating bug", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(parentFeatureID)

	featureData := feature{}

	err = row.Scan(
		&featureData.ID,
		&featureData.Name,
		&featureData.Description,
		&featureData.UserID,
		&featureData.CreatedAt,
		&featureData.UpdatedAt,
	)

	if err != nil {
		s.log.Error("Error bugs.create.scan.", err)

		http.Error(w, "Error creating bug.", http.StatusInternalServerError)

		return
	}

	users, _ := getUsers(s.db)

	pageData := page{Title: "Log a Bug for " + featureData.Name, Data: struct {
		Feature feature
		Users   []user
	}{Feature: featureData, Users: users}}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/bugs/new.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *bugService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	bug_id := ps.ByName("bug_id")

	query := `
SELECT
s.id,
s.name,
s.description,
s.feature_id,
s.user_id,
s.assignee_id,
s.created_at,
s.updated_at,
f.id,
f.name
FROM goissuez.bugs s
JOIN goissuez.features f
ON f.id = s.feature_id
WHERE s.id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error bugs.show.prepare.", err)

		http.Error(w, "Error getting bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(bug_id)

	bugData := bug{Feature: &feature{}}

	// this could be null if there is no assignee
	var assigneeID sql.NullInt64

	err = row.Scan(
		&bugData.ID,
		&bugData.Name,
		&bugData.Description,
		&bugData.FeatureID,
		&bugData.UserID,
		&assigneeID,
		&bugData.CreatedAt,
		&bugData.UpdatedAt,
		&bugData.Feature.ID,
		&bugData.Feature.Name,
	)

	if err != nil {
		s.log.Error("Error bugs.show.scan.", err)

		http.Error(w, "Error getting bug.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		bugData.AssigneeID = assigneeID.Int64

		id := strconv.FormatInt(assigneeID.Int64, 10)

		assignee, err := getUserByID(s.db, id)

		if err != nil {
			s.log.Error("Error bugs.show.getUserByID", err)
		} else {
			bugData.Assignee = &assignee
		}
	}

	creator, err := getUserByID(s.db, strconv.FormatInt(bugData.UserID, 10))

	if err != nil {
		s.log.Error("Error bugs.show.getUserByID", err)
	} else {
		bugData.Creator = &creator
	}

	pageData := page{Title: "Bug Details", Data: bugData, Funcs: make(map[string]interface{})}

	pageData.Funcs["ToJSON"] = func(bugData bug) string {
		b, err := json.Marshal(bugData)

		if err != nil {
			return ""
		}

		return string(b)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/bugs/bug.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *bugService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bug_id := ps.ByName("bug_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.bugs WHERE id = $1`)

	if err != nil {
		s.log.Error("Error bugs.destroy.prepare.", err)

		http.Error(w, "Error deleting bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(bug_id)

	if err != nil {
		s.log.Error("Error bugs.destroy.exec.", err)

		http.Error(w, "Error deleting bug.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
