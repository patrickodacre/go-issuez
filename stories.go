package main

import (
	"database/sql"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strconv"

	"encoding/json"
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
s.id,
s.name,
s.feature_id,
s.user_id,
s.assignee_id,
s.created_at,
s.updated_at,
f.name as feature_name
FROM goissuez.stories s
JOIN goissuez.features f
ON f.id = s.feature_id
ORDER BY s.updated_at
`)

	if err != nil {
		s.log.Error("Error stories.all.prepare.", err)
		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		s.log.Error("Error stories.all.query.", err)
		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {
		storyData := story{Feature: &feature{}}

		var assignee_id sql.NullInt64

		err := rows.Scan(
			&storyData.ID,
			&storyData.Name,
			&storyData.FeatureID,
			&storyData.UserID,
			&assignee_id,
			&storyData.CreatedAt,
			&storyData.UpdatedAt,
			&storyData.Feature.Name,
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

		return
	}

	view.send(http.StatusOK)
}

// Show all stories for a given feature.
// First, we'll get the feature details, and then
// we'll query the related stories separately.
func (s *storyService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	featureData := feature{}

	parentFeatureID := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`SELECT id, name, description, project_id, deleted_at FROM goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Error("Error stories.index.prepare.feature.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(parentFeatureID)

	deleted_at := sql.NullString{}

	err = row.Scan(
		&featureData.ID,
		&featureData.Name,
		&featureData.Description,
		&featureData.ProjectID,
		&deleted_at,
	)

	if err != nil {
		s.log.Error("Error stories.index.scan.feature.", err)

		http.Error(w, "Error listing stories.", http.StatusInternalServerError)

		return
	}

	if deleted_at.Valid {
		deletedEntityNotice("This feature has been deleted, so you cannot view its stories.", w, r, s.log)
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
AND deleted_at IS NULL
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

		return
	}

	view.send(http.StatusOK)
}

func (s *storyService) store(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	feature_id := ps.ByName("feature_id")

	r.ParseForm()

	name := r.PostForm.Get("name")
	description := r.PostForm.Get("description")
	assignee_id := r.PostForm.Get("assignee_id")

	query := `
INSERT INTO goissuez.stories
(name, description, feature_id, user_id, assignee_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error stories.store.prepare.", err)

		http.Error(w, "Error creating story.", http.StatusInternalServerError)

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
		s.log.Error("Error stories.store.exec.", err)

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
	assignee_id := r.PostForm.Get("assignee_id")

	query := `
UPDATE goissuez.stories
SET name = $2, description = $3, assignee_id = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error stories.update.prepare.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

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

	_, err = stmt.Exec(story_id, name, description, assignee)

	if err != nil {
		s.log.Error("Error stories.update.exec.", err)

		http.Error(w, "Error updating story.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/stories/"+story_id, http.StatusSeeOther)
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

	users, _ := getUsers(s.db)

	pageData := page{
		Title: "Edit Story - " + storyData.Name,
		Data: struct {
			Story story
			Users []user
		}{
			storyData,
			users,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/edit.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
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

	users, _ := getUsers(s.db)

	pageData := page{Title: "Create a Story for " + featureData.Name, Data: struct {
		Feature feature
		Users   []user
	}{Feature: featureData, Users: users}}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/new.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
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
f.id,
f.name
FROM goissuez.stories s
JOIN goissuez.features f
ON f.id = s.feature_id
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

	storyData := story{Feature: &feature{}}

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
		&storyData.Feature.ID,
		&storyData.Feature.Name,
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

	if err != nil {
		s.log.Error("Error stories.show.getUserByID", err)
	} else {
		storyData.Creator = &creator
	}

	pageData := page{Title: "Story Details", Data: storyData, Funcs: make(map[string]interface{})}

	pageData.Funcs["ToJSON"] = func(storyData story) string {
		b, err := json.Marshal(storyData)

		if err != nil {
			return ""
		}

		return string(b)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/stories/story.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *storyService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	story_id := ps.ByName("story_id")

	stmt, err := s.db.Prepare(`UPDATE goissuez.stories SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1`)

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
