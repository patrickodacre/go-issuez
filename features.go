package main

import (
	"database/sql"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type featureService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type feature struct {
	ID          int64
	Name        string
	Description string
	ProjectID   int64
	UserID      int64
	CreatedAt   string
	UpdatedAt   string
	Project     project
	Stories     []story
	Bugs        []bug
}

func NewFeatureService(db *sql.DB, log *logrus.Logger, tpls *template.Template) *featureService {
	return &featureService{db, log, tpls}
}

// Show all features for a given project.
// First, we'll get the project details, and then
// we'll query the related features separately.
func (s *featureService) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	projectData := project{}

	parentProjectID := ps.ByName("project_id")

	stmt, err := s.db.Prepare(`SELECT p.id, p.name, p.description FROM goissuez.projects p WHERE p.id = $1`)

	if err != nil {
		s.log.Error("Error features.index.prepare.project.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	err = stmt.QueryRow(parentProjectID).Scan(&projectData.ID, &projectData.Name, &projectData.Description)

	if err != nil {
		s.log.Error("Error features.index.scan.project.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	sql := `
SELECT f.id,
f.name,
f.description,
f.project_id,
f.user_id,
f.created_at,
f.updated_at
FROM goissuez.features f
WHERE f.project_id = $1
ORDER BY f.created_at
`
	stmt, err = s.db.Prepare(sql)

	if err != nil {

		s.log.Error("Error features.index.prepare.features", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(parentProjectID)

	if err != nil {

		s.log.Error("Error features.index.query.", err)

		http.Error(w, "Error listing features.", http.StatusInternalServerError)

		return
	}

	for rows.Next() {

		featureData := &feature{}

		err := rows.Scan(
			&featureData.ID,
			&featureData.Name,
			&featureData.Description,
			&featureData.ProjectID,
			&featureData.UserID,
			&featureData.CreatedAt,
			&featureData.UpdatedAt,
		)

		if err != nil {

			s.log.Error("Error features.index.scan.", err)

			http.Error(w, "Error listing features.", http.StatusInternalServerError)

			return
		}

		projectData.Features = append(projectData.Features, *featureData)
	}

	pageData := page{Title: "Features", Data: projectData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/features/features.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

// Save a project feature.
func (s *featureService) store(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	project_id := ps.ByName("project_id")

	r.ParseForm()

	featureName := r.PostForm.Get("name")
	featureDescription := r.PostForm.Get("description")

	sql := `
INSERT INTO goissuez.features
(name, description, project_id, user_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Error("Error features.store.prepare.", err)

		http.Error(w, "Error creating feature.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(featureName, featureDescription, project_id, authUser.ID)

	if err != nil {
		s.log.Error("Error features.store.queryrow.", err)

		http.Error(w, "Error saving feature.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/projects/"+project_id, http.StatusSeeOther)
}

// Update a project feature.
func (s *featureService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	feature_id := ps.ByName("feature_id")

	r.ParseForm()

	featureName := r.PostForm.Get("name")
	featureDescription := r.PostForm.Get("description")

	// return the project_id so we can redirect back to the project / features page
	sql := `
UPDATE goissuez.features
SET name = $2, description = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING project_id
`

	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Error("Error features.update.prepare.", err)

		http.Error(w, "Error updating feature.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		project_id string
	)

	err = stmt.QueryRow(feature_id, featureName, featureDescription).Scan(&project_id)

	if err != nil {
		s.log.Error("Error features.update.exec.", err)

		http.Error(w, "Error updating feature.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/projects/"+project_id+"/features", http.StatusSeeOther)
}

// Show the edit feature form.
func (s *featureService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	feature_id := ps.ByName("feature_id")

	sql := `
SELECT
id,
name,
description,
project_id,
user_id,
created_at,
updated_at
FROM goissuez.features
WHERE id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Error("Error features.edit.prepare.", err)

		http.Error(w, "Error editing feature.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(feature_id)

	featureData := feature{}

	err = row.Scan(
		&featureData.ID,
		&featureData.Name,
		&featureData.Description,
		&featureData.ProjectID,
		&featureData.UserID,
		&featureData.CreatedAt,
		&featureData.UpdatedAt,
	)

	if err != nil {
		s.log.Error("Error features.edit.scan.", err)

		http.Error(w, "Error editing feature.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Edit Feature", Data: featureData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/features/edit.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

// Show the new / create feature form.
func (s *featureService) create(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	parentProjectID := ps.ByName("project_id")

	sql := `
SELECT
id,
name,
description,
user_id,
created_at,
updated_at
FROM goissuez.projects
WHERE id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Error("Error features.create.prepare.", err)

		http.Error(w, "Error creating feature", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(parentProjectID)

	projectData := project{}

	err = row.Scan(
		&projectData.ID,
		&projectData.Name,
		&projectData.Description,
		&projectData.UserID,
		&projectData.CreatedAt,
		&projectData.UpdatedAt,
	)

	if err != nil {
		s.log.Error("Error features.create.scan.", err)

		http.Error(w, "Error creating feature.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Create " + projectData.Name + " Feature ", Data: projectData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/features/new.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *featureService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	feature_id := ps.ByName("feature_id")
	stories := []story{}
	bugs := []bug{}

	sql := `
SELECT
f.id,
f.name,
f.description,
f.project_id,
f.user_id,
f.created_at,
f.updated_at,
p.name as project_name
FROM goissuez.features f
JOIN goissuez.projects p
ON p.id = f.project_id
WHERE f.id = $1
LIMIT 1
`

	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Error("Error features.show.prepare.", err)

		http.Error(w, "Error getting feature.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(feature_id)

	featureData := feature{}

	err = row.Scan(
		&featureData.ID,
		&featureData.Name,
		&featureData.Description,
		&featureData.ProjectID,
		&featureData.UserID,
		&featureData.CreatedAt,
		&featureData.UpdatedAt,
		&featureData.Project.Name,
	)

	if err != nil {
		s.log.Error("Error features.show.scan.", err)

		http.Error(w, "Error getting feature.", http.StatusInternalServerError)

		return
	}

	// get the stories for the project
	{
		stmt, err := s.db.Prepare(`
			SELECT
			id,
			name,
			feature_id,
			created_at,
			updated_at
			FROM goissuez.stories
			WHERE feature_id = $1
		`)

		if err != nil {
			s.log.Error("Error features.show.prepare.stories.", err)

			http.Error(w, "Error getting feature stories.", http.StatusInternalServerError)

			return
		}

		rows, err := stmt.Query(feature_id)

		if err != nil {
			s.log.Error("Error features.show.query.stories.", err)

			http.Error(w, "Error getting feature stories.", http.StatusInternalServerError)

			return
		}

		for rows.Next() {

			storyData := story{}

			err := rows.Scan(
				&storyData.ID,
				&storyData.Name,
				&storyData.FeatureID,
				&storyData.CreatedAt,
				&storyData.UpdatedAt,
			)

			if err != nil {

				s.log.Error("Error features.show.scan.stories.", err)

				http.Error(w, "Error getting feature stories.", http.StatusInternalServerError)

				return
			}

			stories = append(stories, storyData)
		}

		featureData.Stories = stories
	}

	// get the bugs for the project
	{
		stmt, err := s.db.Prepare(`
			SELECT
			id,
			name,
			feature_id,
			created_at,
			updated_at
			FROM goissuez.bugs
			WHERE feature_id = $1
		`)

		if err != nil {
			s.log.Error("Error features.show.prepare.bugs.", err)

			http.Error(w, "Error getting feature bugs.", http.StatusInternalServerError)

			return
		}

		rows, err := stmt.Query(feature_id)

		if err != nil {
			s.log.Error("Error features.show.query.bugs.", err)

			http.Error(w, "Error getting feature bugs.", http.StatusInternalServerError)

			return
		}

		for rows.Next() {

			bugData := bug{}

			err := rows.Scan(
				&bugData.ID,
				&bugData.Name,
				&bugData.FeatureID,
				&bugData.CreatedAt,
				&bugData.UpdatedAt,
			)

			if err != nil {

				s.log.Error("Error features.show.scan.bugs.", err)

				http.Error(w, "Error getting feature bugs.", http.StatusInternalServerError)

				return
			}

			bugs = append(bugs, bugData)
		}

		featureData.Bugs = bugs
	}

	pageData := page{Title: featureData.Name, Data: featureData, Funcs: make(map[string]interface{})}

	pageData.Funcs["ToJSON"] = func(data interface{}) string {

		var storyData story
		var bugData bug
		var b []byte
		var err error

		bugData, bug_ok := data.(bug)
		storyData, story_ok := data.(story)

		if bug_ok {
			b, err = json.Marshal(bugData)
		} else if story_ok {
			b, err = json.Marshal(storyData)
		}

		if err != nil {
			return ""
		}

		return string(b)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/features/feature.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

func (s *featureService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	feature_id := ps.ByName("feature_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.features WHERE id = $1`)

	if err != nil {
		s.log.Error("Error features.destroy.prepare.", err)

		http.Error(w, "Error deleting feature.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(feature_id)

	if err != nil {
		s.log.Error("Error features.destroy.exec.", err)

		http.Error(w, "Error deleting feature.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
