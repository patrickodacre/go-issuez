package main

import (
	"database/sql"
	"html/template"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

type storyService struct {
	db   *sql.DB
	log  *logrus.Logger
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
	Creator     *user
	Assignee    *user
	Feature     *feature
	Project     *project
}

func NewStoryService(db *sql.DB, log *logrus.Logger, tpls *template.Template) *storyService {
	return &storyService{db, log, tpls}
}

func (s *storyService) all(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	stories := []story{}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
user_id,
assignee_id,
created_at,
updated_at
FROM goissuez.stories
ORDER BY updated_at
`)

	if err != nil {
		s.log.Error("Error stories.all.prepare.", err)
		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	rows, err := stmt.Query()

	if err != nil {
		s.log.Error("Error stories.all.query.", err)
		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {
		storyData := story{}

		var assignee_id sql.NullInt64

		err := rows.Scan(
			&storyData.ID,
			&storyData.Name,
			&storyData.UserID,
			&assignee_id,
			&storyData.CreatedAt,
			&storyData.UpdatedAt,
		)

		if err != nil {
			s.log.Error("Error stories.all.scan.", err)
			http.Error(w, "Error listing stories.", http.StatusInternalServerError)

			return
		}

		if assignee_id.Valid {
			storyData.AssigneeID = assignee_id.Int64
		}

		stories = append(stories, storyData)
	}

	users, err := getUsers(s.db)

	usersByID := make(map[int64]*user)

	if err == nil {
		for i := 0; i < len(users); i++ {
			usersByID[users[i].ID] = &users[i]
		}
	}

	for i := 0; i < len(stories); i++ {

		// make sure we're mutating the actual story
		// in the slice
		story := &stories[i]

		creator, ok := usersByID[story.UserID]

		if ok {
			story.Creator = creator
		}

		assignee, ok := usersByID[story.AssigneeID]

		if ok {
			story.Assignee = assignee
		}
	}

	pageData := page{
		Title: "Stories",
		Data: struct {
			Stories []story
		}{
			stories,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/all.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

// Show all stories for a given feature.
// First, we'll get the feature details, and then
// we'll query the related stories separately.
func (s *storyService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	featureData := feature{}

	parentFeatureID := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`SELECT id, name, description, project_id FROM goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Error("Error stories.index.prepare.feature.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	err = stmt.QueryRow(parentFeatureID).Scan(&featureData.ID, &featureData.Name, &featureData.Description, &featureData.ProjectID)

	if err != nil {
		s.log.Error("Error stories.index.scan.feature.", err)

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

		s.log.Error("Error stories.index.prepare.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(parentFeatureID)

	if err != nil {

		s.log.Error("Error stories.index.query.", err)

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

			s.log.Error("Error stories.index.scan.", err)

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

	view := viewService{w: w, r: r}
	view.make("templates/stories/stories.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
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
		s.log.Error("Error stories.store.prepare.", err)

		http.Error(w, "Error creating story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(name, description, feature_id, authUser.ID)

	if err != nil {
		s.log.Error("Error stories.store.queryrow.", err)

		http.Error(w, "Error saving story.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/features/"+feature_id, http.StatusSeeOther)
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
		s.log.Error("Error stories.update.prepare.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		feature_id string
	)

	err = stmt.QueryRow(story_id, name, description).Scan(&feature_id)

	if err != nil {
		s.log.Error("Error stories.update.exec.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/features/"+feature_id, http.StatusSeeOther)
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
		s.log.Error("Error stories.edit.prepare.", err)

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
		s.log.Error("Error stories.edit.scan.", err)

		http.Error(w, "Error editing story.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		storyData.AssigneeID = assigneeID.Int64
	}

	pageData := page{Title: "Story Details", Data: storyData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/edit.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
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
		s.log.Error("Error stories.create.prepare.", err)

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
		s.log.Error("Error stories.create.scan.", err)

		http.Error(w, "Error creating story.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Story Details", Data: featureData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/new.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
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
s.updated_at
FROM goissuez.stories s
WHERE s.id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error stories.show.prepare.", err)

		http.Error(w, "Error getting story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(story_id)

	storyData := story{}

	// this could be null if there is no assignee
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
		s.log.Error("Error stories.show.scan.", err)

		http.Error(w, "Error getting story.", http.StatusInternalServerError)

		return
	}

	if assigneeID.Valid {
		storyData.AssigneeID = assigneeID.Int64

		id := strconv.FormatInt(assigneeID.Int64, 10)

		assignee, err := getUserByID(s.db, id)

		if err != nil {
			s.log.Error("Error stories.show.getUserByID", err)
		} else {
			storyData.Assignee = &assignee
		}
	}

	creator, err := getUserByID(s.db, strconv.FormatInt(storyData.UserID, 10))

	storyData.Creator = &creator

	pageData := page{Title: "Story Details", Data: storyData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/story.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *storyService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	story_id := ps.ByName("story_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.stories WHERE id = $1`)

	if err != nil {
		s.log.Error("Error stories.destroy.prepare.", err)

		http.Error(w, "Error deleting story.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(story_id)

	if err != nil {
		s.log.Error("Error stories.destroy.exec.", err)

		http.Error(w, "Error deleting story.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
