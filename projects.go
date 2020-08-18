package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type projectService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type project struct {
	ID          int64
	Name        string
	Description string
	UserID      int64
	CreatedAt   string
	UpdatedAt   string
	Features    []feature
}

func NewProjectService(db *sql.DB, log *logrus.Logger, tpls *template.Template) *projectService {
	return &projectService{db, log, tpls}
}

func (s *projectService) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"read_projects_mine"}) && !authUser.Can([]string{"read_projects_others"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := `
SELECT
id,
name,
description,
user_id,
created_at,
updated_at
FROM goissuez.projects
WHERE deleted_at IS NULL
ORDER BY created_at
`
	stmt, err := s.db.Prepare(query)

	if err != nil {

		s.log.Error("Error projects.index.prepare.", err)

		http.Error(w, "Error listing projects.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {

		s.log.Error("Error projects.index.query.", err)

		http.Error(w, "Error listing projects.", http.StatusInternalServerError)

		return
	}

	projects := []project{}

	for rows.Next() {

		projectData := project{}
		description := sql.NullString{}

		err := rows.Scan(
			&projectData.ID,
			&projectData.Name,
			&description,
			&projectData.UserID,
			&projectData.CreatedAt,
			&projectData.UpdatedAt,
		)

		if err != nil {

			s.log.Error("Error projects.index.scan.", err)

			http.Error(w, "Error listing projects.", http.StatusInternalServerError)

			return
		}

		if description.Valid {
			projectData.Description = description.String
		}

		projects = append(projects, projectData)
	}

	filteredProjectsList := []project{}

	for _, p := range projects {
		if p.ID == 2 {
			p.Name = "Admin Demo Project"
			p.Description = "This project was created by the Demo Admin. It can be edited by the Demo Admin, though changes won't appear to persist."
		}

		if p.UserID == authUser.ID && authUser.Can([]string{"read_projects_mine"}) {
			filteredProjectsList = append(filteredProjectsList, p)
		} else if p.UserID != authUser.ID && authUser.Can([]string{"read_projects_others"}) {
			filteredProjectsList = append(filteredProjectsList, p)
		}
	}

	pageData := page{Title: "All Projects", Data: struct{ Projects []project }{filteredProjectsList}}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/projects/projects.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *projectService) store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.Can([]string{"create_projects"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	query := `
INSERT INTO goissuez.projects
(name, description, user_id, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error projects.store.prepare.", err)

		http.Error(w, "Error creating project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		id int64
	)

	err = stmt.QueryRow(projectName, projectDescription, authUser.ID).Scan(&id)

	if err != nil {
		s.log.Error("Error projects.store.queryrow.", err)

		http.Error(w, "Error saving project.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *projectService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	project_id := ps.ByName("project_id")

	// ensure we have the correct permissions
	{
		projectData := project{}

		stmt, err := s.db.Prepare(`SELECT user_id FROM goissuez.projects WHERE id = $1`)

		if err != nil {
			s.log.Error("Error projects.update.getproject.prepare.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		err = stmt.QueryRow(project_id).Scan(
			&projectData.UserID,
		)

		if err != nil {
			s.log.Error("Error projects.update.getproject.scan.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		if projectData.UserID == authUser.ID && !authUser.Can([]string{"update_projects_mine"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else if projectData.UserID != authUser.ID && !authUser.Can([]string{"update_projects_others"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	query := `
UPDATE goissuez.projects
SET
name = $2,
description = $3,
updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error projects.update.prepare.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(project_id, projectName, projectDescription)

	if err != nil {
		s.log.Error("Error projects.update.exec.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/projects/"+project_id, http.StatusSeeOther)
}

func (s *projectService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	project_id := ps.ByName("project_id")

	// ensure we have the correct permissions
	{
		projectData := project{}

		stmt, err := s.db.Prepare(`SELECT user_id FROM goissuez.projects WHERE id = $1`)

		if err != nil {
			s.log.Error("Error projects.edit.getproject.prepare.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		err = stmt.QueryRow(project_id).Scan(
			&projectData.UserID,
		)

		if err != nil {
			s.log.Error("Error projects.edit.getproject.scan.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		if projectData.UserID == authUser.ID && !authUser.Can([]string{"update_projects_mine"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else if projectData.UserID != authUser.ID && !authUser.Can([]string{"update_projects_others"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	query := `
SELECT
id,
name,
description,
user_id
FROM goissuez.projects
WHERE id = $1
LIMIT 1
`
	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error projects.edit.prepare.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id)

	projectData := project{}
	description := sql.NullString{}

	err = row.Scan(
		&projectData.ID,
		&projectData.Name,
		&description,
		&projectData.UserID,
	)

	if err != nil {
		s.log.Error("Error projects.edit.scan.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	if description.Valid {
		projectData.Description = description.String
	}

	pageData := page{Title: "Edit Project", Data: projectData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/projects/edit.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *projectService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	project_id := ps.ByName("project_id")

	if project_id == "new" {

		if !authUser.Can([]string{"create_projects"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		pageData := page{Title: "Create Project", Data: nil}

		w.Header().Set("Content-Type", "text/html; charset=UTF-8")

		view := viewService{w: w, r: r}
		view.make("templates/projects/new.gohtml")
		err := view.exec(mainLayout, pageData)

		if err != nil {
			s.log.Error(err)
			http.Error(w, "Error", http.StatusInternalServerError)

			return
		}

		view.send(http.StatusOK)
		return
	}

	// Get Project Details
	query := `
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

	stmt, err := s.db.Prepare(query)

	if err != nil {
		s.log.Error("Error projects.show.prepare.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id)

	projectData := project{}
	description := sql.NullString{}

	err = row.Scan(
		&projectData.ID,
		&projectData.Name,
		&description,
		&projectData.UserID,
		&projectData.CreatedAt,
		&projectData.UpdatedAt,
	)

	if err != nil {
		s.log.Error("Error projects.show.scan.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	if description.Valid {
		projectData.Description = description.String
	}

	// hack to ensure demo project description and name doesn't change
	if projectData.ID == 2 {

		projectData.Name = "Admin Demo Project"
		projectData.Description = "This project was created by the Demo Admin. It can be edited by the Demo Admin, though changes won't appear to persist."
	}

	if projectData.UserID == authUser.ID {
		if !authUser.Can([]string{"read_projects_mine"}) {

			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}

	} else {
		if !authUser.Can([]string{"read_projects_others"}) {

			http.Error(w, "Unauthorized", http.StatusUnauthorized)

			return
		}
	}

	// Get Features for the Project

	stmt, err = s.db.Prepare(`
SELECT
id,
name,
description,
project_id,
user_id,
created_at,
updated_at
FROM goissuez.features
WHERE project_id = $1
`)

	if err != nil {
		s.log.Error("Error projects.show.prepare.features.", err)

		http.Error(w, "Error getting project features.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(project_id)

	if err != nil {
		s.log.Error("Error projects.show.query.features.", err)

		http.Error(w, "Error getting project features.", http.StatusInternalServerError)

		return
	}

	features := []feature{}

	for rows.Next() {
		featureData := feature{}
		description := sql.NullString{}

		err := rows.Scan(
			&featureData.ID,
			&featureData.Name,
			&description,
			&featureData.ProjectID,
			&featureData.UserID,
			&featureData.CreatedAt,
			&featureData.UpdatedAt,
		)

		if err != nil {

			s.log.Error("Error projects.show.scan.features.", err)

			http.Error(w, "Error getting project features.", http.StatusInternalServerError)

			return
		}

		if description.Valid {
			featureData.Description = description.String
		}

		features = append(features, featureData)
	}

	projectData.Features = features

	pageData := page{Title: projectData.Name, Data: projectData, Funcs: make(map[string]interface{})}

	pageData.Funcs["ToJSON"] = func(data interface{}) string {

		var featureData feature
		var projectData project
		var b []byte
		var err error

		featureData, feature_ok := data.(feature)
		projectData, project_ok := data.(project)

		if feature_ok {
			b, err = json.Marshal(featureData)
		} else if project_ok {
			b, err = json.Marshal(projectData)
		}

		if err != nil {
			return ""
		}

		return string(b)
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	view := viewService{w: w, r: r}
	view.make("templates/projects/project.gohtml")

	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *projectService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	project_id := ps.ByName("project_id")

	// ensure we have the correct permissions
	{
		projectData := project{}

		stmt, err := s.db.Prepare(`SELECT user_id FROM goissuez.projects WHERE id = $1`)

		if err != nil {
			s.log.Error("Error projects.destroy.getproject.prepare.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		err = stmt.QueryRow(project_id).Scan(
			&projectData.UserID,
		)

		if err != nil {
			s.log.Error("Error projects.destroy.getproject.scan.", err)

			http.Error(w, "Error updating project.", http.StatusInternalServerError)
			return
		}

		if projectData.UserID == authUser.ID && !authUser.Can([]string{"delete_projects_mine"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else if projectData.UserID != authUser.ID && !authUser.Can([]string{"delete_projects_others"}) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	tx, err := s.db.BeginTx(context.Background(), nil)

	if err != nil {
		s.log.Error("Error projects.destroy.begintx.", err)

		http.Error(w, "Cannot delete project.", http.StatusInternalServerError)
		return
	}

	// DELETE Project
	{
		stmt, err := tx.Prepare(`UPDATE goissuez.projects SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1`)

		if err != nil {
			s.log.Error("Error projects.destroy.prepare.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(project_id)

		if err != nil {
			s.log.Error("Error projects.destroy.exec.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}
	}

	// Compile a list of features that we'll be deleting.
	// we'll need to reference this list of ids to delete associated bugs and stories.
	feature_ids := []int64{}
	{
		// first save all feature IDs to make handling stories and bugs easier
		stmt, err := tx.Prepare(`SELECT id from goissuez.features WHERE project_id = $1`)

		if err != nil {
			s.log.Error("Error projects.destroy.query.features.prepare.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		defer stmt.Close()

		rows, err := stmt.Query(project_id)

		if err != nil {
			s.log.Error("Error projects.destroy.query.features.query.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		for rows.Next() {
			var id int64

			err := rows.Scan(
				&id,
			)

			if err != nil {
				s.log.Error("Error projects.destroy.query.features.scan.", err)

				tx.Rollback()
				http.Error(w, "Error deleting project.", http.StatusInternalServerError)
				return
			}

			feature_ids = append(feature_ids, id)
		}
	}

	// DELETE Features
	{
		stmt, err := tx.Prepare(`UPDATE goissuez.features SET deleted_at = CURRENT_TIMESTAMP WHERE project_id = $1`)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.features.prepare.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(project_id)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.features.exec.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

	}

	// create a string of feature ids we can reference to delete associated bugs and stories.
	// why not just use postgres to cascade these changes? b/c we're using soft-deletes.
	var deleted_feature_ids string
	{
		feature_id_strings := []string{}

		for _, v := range feature_ids {
			feature_id_strings = append(feature_id_strings, strconv.FormatInt(v, 10))
		}

		deleted_feature_ids = strings.Join(feature_id_strings, ", ")
	}

	// DELETE stories
	{
		stmt, err := tx.Prepare(`UPDATE goissuez.stories SET deleted_at = CURRENT_TIMESTAMP WHERE feature_id IN ($1)`)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.stories.prepare.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(deleted_feature_ids)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.stories.exec.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}
	}

	// DELETE Bugs
	{
		stmt, err := tx.Prepare(`UPDATE goissuez.bugs SET deleted_at = CURRENT_TIMESTAMP WHERE feature_id IN ($1)`)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.bugs..prepare.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}

		defer stmt.Close()

		_, err = stmt.Exec(deleted_feature_ids)

		if err != nil {
			s.log.Error("Error projects.destroy.delete.bugs.exec.", err)

			tx.Rollback()
			http.Error(w, "Error deleting project.", http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
