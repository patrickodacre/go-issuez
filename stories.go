package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type storyService struct {
	db   *sql.DB
	log  *log.Logger
	tpls *template.Template
}

type story struct {
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

func NewStoryService(db *sql.DB, log *log.Logger, tpls *template.Template) *storyService {
	return &storyService{db, log, tpls}
}

// Show all stories for a given feature.
// First, we'll get the feature details, and then
// we'll query the related stories separately.
func (s *storyService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	featureData := feature{}

	parentFeatureID := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`SELECT id, name, description, project_id FROM goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Println("Error stories.index.prepare.feature.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	err = stmt.QueryRow(parentFeatureID).Scan(&featureData.ID, &featureData.Name, &featureData.Description, &featureData.ProjectID)

	if err != nil {
		s.log.Println("Error stories.index.scan.feature.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

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
FROM goissuez.stories
WHERE feature_id = $1
ORDER BY created_at
`
	stmt, err = s.db.Prepare(query)

	if err != nil {

		s.log.Println("Error stories.index.prepare.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(parentFeatureID)

	if err != nil {

		s.log.Println("Error stories.index.query.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		storyData := &story{}
		var assigneeID sql.NullInt64

		err := rows.Scan(
			&storyData.ID,
			&storyData.Name,
			&storyData.Description,
			&storyData.FeatureID,
			&storyData.UserID,
			&assigneeID,
			&storyData.CreatedAt,
			&storyData.UpdatedAt,
		)

		if err != nil {

			s.log.Println("Error stories.index.scan.", err)

			http.Error(w, "Error listing stories.", http.StatusInternalServerError)

			return
		}

		if assigneeID.Valid {
			storyData.AssigneeID = assigneeID.Int64
		}

		featureData.Stories = append(featureData.Stories, *storyData)
	}

	pageData := page{Title: "Stories", Data: featureData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, log: s.log}
	view.make("templates/stories/stories.gohtml")
	view.exec("main_layout", pageData)
}

func (s *storyService) store(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	feature_id := ps.ByName("feature_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")

	query := `
INSERT INTO goissuez.stories
(name, description, feature_id, user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error stories.store.prepare.", err)

		http.Error(w, "Error creating story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(name, description, feature_id, authUser.ID)

	if err != nil {
		s.log.Println("Error stories.store.queryrow.", err)

		http.Error(w, "Error saving story.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/features/"+feature_id+"/stories", http.StatusSeeOther)
}

func (s *storyService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	story_id := ps.ByName("story_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")

	// return the feature_id so we can redirect back to the feature / stories page
	query := `
UPDATE goissuez.stories
SET name = $2, description = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING feature_id
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error stories.update.prepare.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		feature_id string
	)

	err = stmt.QueryRow(story_id, name, description).Scan(&feature_id)

	if err != nil {
		s.log.Println("Error stories.update.exec.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/features/"+feature_id+"/stories", http.StatusSeeOther)
}

func (s *storyService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	story_id := ps.ByName("story_id")

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

FROM goissuez.stories
WHERE id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Println("Error stories.edit.prepare.", err)

		http.Error(w, "Error editing story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(story_id)

	storyData := story{}
	var assigneeID sql.NullInt64

	err = row.Scan(
		&storyData.ID,
		&storyData.Name,
		&storyData.Description,
		&storyData.FeatureID,
		&storyData.UserID,
		&assigneeID,
		&storyData.CreatedAt,
		&storyData.UpdatedAt,
	)

	if err != nil {
		s.log.Println("Error stories.edit.scan.", err)

		http.Error(w, "Error editing story.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		storyData.AssigneeID = assigneeID.Int64
	}

	pageData := page{Title: "Story Details", Data: storyData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, log: s.log}
	view.make("templates/stories/edit.gohtml")
	view.exec("main_layout", pageData)
}

// Show the new / create feature form.
func (s *storyService) create(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

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
		s.log.Println("Error stories.create.prepare.", err)

		http.Error(w, "Error creating story", http.StatusInternalServerError)

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
		s.log.Println("Error stories.create.scan.", err)

		http.Error(w, "Error creating story.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Story Details", Data: featureData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, log: s.log}
	view.make("templates/stories/new.gohtml")
	view.exec("main_layout", pageData)
}

func (s *storyService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	story_id := ps.ByName("story_id")

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

FROM goissuez.stories s

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
		s.log.Println("Error stories.show.prepare.", err)

		http.Error(w, "Error getting story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(story_id)

	storyData := story{}

	// this could be null if there is no assignee
	var assigneeName sql.NullString
	var assigneeID sql.NullInt64

	err = row.Scan(
		&storyData.ID,
		&storyData.Name,
		&storyData.Description,
		&storyData.FeatureID,
		&storyData.UserID,
		&assigneeID,
		&storyData.CreatedAt,
		&storyData.UpdatedAt,
		&storyData.Feature.Name,
		&storyData.Creator.Name,
		&assigneeName,
	)

	if err != nil {
		s.log.Println("Error stories.show.scan.", err)

		http.Error(w, "Error getting story.", http.StatusInternalServerError)

		return
	}

	if assigneeName.Valid {
		storyData.Assignee.Name = assigneeName.String
	} else {
		storyData.Assignee.Name = "Not Assigned"
	}

	if assigneeID.Valid {
		storyData.AssigneeID = assigneeID.Int64
	}

	pageData := page{Title: "Story Details", Data: storyData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, log: s.log}
	view.make("templates/stories/story.gohtml")
	view.exec("main_layout", pageData)
}

func (s *storyService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	story_id := ps.ByName("story_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.stories WHERE id = $1`)

	if err != nil {
		s.log.Println("Error stories.destroy.prepare.", err)

		http.Error(w, "Error deleting story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(story_id)

	if err != nil {
		s.log.Println("Error stories.destroy.exec.", err)

		http.Error(w, "Error deleting story.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
