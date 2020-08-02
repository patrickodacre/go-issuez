package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type projectService struct {
	db   *sql.DB
	log  *log.Logger
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

func NewProjectService(db *sql.DB, log *log.Logger, tpls *template.Template) *projectService {
	return &projectService{db, log, tpls}
}

func (s *projectService) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	sql := `
SELECT
id,
name,
description,
user_id,
created_at,
updated_at
FROM goissuez.projects
WHERE user_id = $1
ORDER BY created_at
`
	stmt, err := s.db.Prepare(sql)

	if err != nil {

		s.log.Println("Error projects.index.prepare.", err)

		http.Error(w, "Error listing projects.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query(authUser.ID)

	if err != nil {

		s.log.Println("Error projects.index.query.", err)

		http.Error(w, "Error listing projects.", http.StatusInternalServerError)

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

			s.log.Println("Error projects.index.scan.", err)

			http.Error(w, "Error listing projects.", http.StatusInternalServerError)

			return
		}

		projects = append(projects, projectData)
	}

	pageData := page{Title: "Projects", Data: struct{Projects []project}{projects}}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, r: r}
	view.make("templates/projects/projects.gohtml")
	err = view.exec("main_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)

		return
	}
}

func (s *projectService) store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	sql := `
INSERT INTO goissuez.projects
(name, description, user_id, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING id
`
	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Println("Error projects.store.prepare.", err)

		http.Error(w, "Error creating project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	var (
		id int64
	)

	err = stmt.QueryRow(projectName, projectDescription, authUser.ID).Scan(&id)

	if err != nil {
		s.log.Println("Error projects.store.queryrow.", err)

		http.Error(w, "Error saving project.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *projectService) update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	project_id := ps.ByName("project_id")

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	sql := `
UPDATE goissuez.projects
SET
name = $2,
description = $3,
updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Println("Error projects.update.prepare.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(project_id, projectName, projectDescription)

	if err != nil {
		s.log.Println("Error projects.update.exec.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/projects/" + project_id, http.StatusSeeOther)
}

func (s *projectService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	project_id := ps.ByName("project_id")

	sql := `
SELECT
id,
name,
description,
user_id
FROM goissuez.projects
WHERE id = $1
LIMIT 1
`
	stmt, err := s.db.Prepare(sql)

	if err != nil {
		s.log.Println("Error projects.edit.prepare.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id)

	projectData := project{}

	err = row.Scan(
		&projectData.ID,
		&projectData.Name,
		&projectData.Description,
		&projectData.UserID,
	)

	if err != nil {
		s.log.Println("Error projects.edit.scan.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Edit Project", Data: projectData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, r: r}
	view.make("templates/projects/edit.gohtml")
	err = view.exec("main_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}
}

func (s *projectService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	project_id := ps.ByName("project_id")

	if project_id == "new" {

		pageData := page{Title: "Create Project", Data: nil}

		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		view := viewService{w: w, r: r}
		view.make("templates/projects/new.gohtml")
		err := view.exec("main_layout", pageData)

		if err != nil {
			s.log.Error(err)
			http.Error(w, "Error", http.StatusInternalServerError)
		}

		return
	}

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
		s.log.Println("Error projects.show.prepare.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id)


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
		s.log.Println("Error projects.show.scan.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	pageData := page{Title: "Project Details", Data: projectData}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	view := viewService{w: w, r: r}
	view.make("templates/projects/project.gohtml")
	err = view.exec("main_layout", pageData)

	if err != nil {
		s.log.Error(err)
		http.Error(w, "Error", http.StatusInternalServerError)
	}
}

func (s *projectService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	project_id := ps.ByName("project_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.projects WHERE id = $1`)

	if err != nil {
		s.log.Println("Error projects.destroy.prepare.", err)

		http.Error(w, "Error deleting project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(project_id)

	if err != nil {
		s.log.Println("Error projects.destroy.exec.", err)

		http.Error(w, "Error deleting project.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}
