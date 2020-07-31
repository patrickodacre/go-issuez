package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type bugService struct {
	db   *sql.DB
	log  *log.Logger
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
	Creator     user
	Assignee    user
	Feature     feature
	Project     project
}

func NewBugService(db *sql.DB, log *log.Logger, tpls *template.Template) *bugService {
	return &bugService{db, log, tpls}
}

func (s *bugService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	featureData := feature{}

	parentFeatureID := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`SELECT id, name, description, project_id FROM goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Println("Error bugs.index.prepare.feature.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	err = stmt.QueryRow(parentFeatureID).Scan(&featureData.ID, &featureData.Name, &featureData.Description, &featureData.ProjectID)

	if err != nil {
		s.log.Println("Error bugs.index.scan.feature.", err)

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

		s.log.Println("Error bugs.index.prepare.", err)

		http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(parentFeatureID)

	if err != nil {

		s.log.Println("Error bugs.index.query.", err)

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

			s.log.Println("Error bugs.index.scan.", err)

			http.Error(w, "Error listing bugs.", http.StatusInternalServerError)

			return
		}

		if assigneeID.Valid {
			bugData.AssigneeID = assigneeID.Int64
		}

		featureData.Bugs = append(featureData.Bugs, *bugData)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	tpls.ExecuteTemplate(w, "bugs/bugs.gohtml", struct{ Feature feature }{featureData})
}

func (s *bugService) store(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	feature_id := ps.ByName("feature_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")

	query := `
INSERT INTO goissuez.bugs
(name, description, feature_id, user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error bugs.store.prepare.", err)

		http.Error(w, "Error creating bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(name, description, feature_id, authUser.ID)

	if err != nil {
		s.log.Println("Error bugs.store.queryrow.", err)

		http.Error(w, "Error saving bug.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/features/"+feature_id+"/bugs", http.StatusSeeOther)
}

func (s *bugService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	bug_id := ps.ByName("bug_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")

	// return the feature_id so we can redirect back to the feature / bugs page
	query := `
UPDATE goissuez.bugs
SET name = $2, description = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING feature_id
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error bugs.update.prepare.", err)

		http.Error(w, "Error updating bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		feature_id string
	)

	err = stmt.QueryRow(bug_id, name, description).Scan(&feature_id)

	if err != nil {
		s.log.Println("Error bugs.update.exec.", err)

		http.Error(w, "Error updating bug.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/features/"+feature_id+"/bugs", http.StatusSeeOther)
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
		s.log.Println("Error bugs.edit.prepare.", err)

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
		s.log.Println("Error bugs.edit.scan.", err)

		http.Error(w, "Error editing bug.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		bugData.AssigneeID = assigneeID.Int64
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s.tpls.ExecuteTemplate(w, "bugs/edit.gohtml", bugData)
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
		s.log.Println("Error bugs.create.prepare.", err)

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
		s.log.Println("Error bugs.create.scan.", err)

		http.Error(w, "Error creating bug.", http.StatusInternalServerError)

		return
	}

	s.tpls.ExecuteTemplate(w, "bugs/new.gohtml", struct{ Feature feature }{featureData})
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
f.name as feature_name,
creator.name as creator_name,
assignee.name as assignee_name

FROM goissuez.bugs s

JOIN goissuez.features as f
on f.id = s.feature_id
LEFT JOIN goissuez.users as creator
on creator.id = s.user_id
LEFT JOIN goissuez.users as assignee
on assignee.id = s.assignee_id

WHERE s.id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error bugs.show.prepare.", err)

		http.Error(w, "Error getting bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(bug_id)

	bugData := bug{}

	// this could be null if there is no assignee
	var assigneeName sql.NullString
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
		&bugData.Feature.Name,
		&bugData.Creator.Name,
		&assigneeName,
	)

	if err != nil {
		s.log.Println("Error bugs.show.scan.", err)

		http.Error(w, "Error getting bug.", http.StatusInternalServerError)

		return
	}

	if assigneeName.Valid {
		bugData.Assignee.Name = assigneeName.String
	} else {
		bugData.Assignee.Name = "Not Assigned"
	}

	if assigneeID.Valid {
		bugData.AssigneeID = assigneeID.Int64
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s.tpls.ExecuteTemplate(w, "bugs/bug.gohtml", bugData)
}

func (s *bugService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bug_id := ps.ByName("bug_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.bugs WHERE id = $1`)

	if err != nil {
		s.log.Println("Error bugs.destroy.prepare.", err)

		http.Error(w, "Error deleting bug.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(bug_id)

	if err != nil {
		s.log.Println("Error bugs.destroy.exec.", err)

		http.Error(w, "Error deleting bug.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
