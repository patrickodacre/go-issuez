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
}

func NewProjectService(db *sql.DB, log *log.Logger, tpls *template.Template) *projectService {
	return &projectService{db, log, tpls}
}

func (s *projectService) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	sql := `
SELECT * FROM goissuez.projects p
WHERE p.user_id = $1
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

	var (
		id          int64
		name        string
		description string
		user_id     int64
		created_at  string
	)

	for rows.Next() {

		err := rows.Scan(&id, &name, &description, &user_id, &created_at)

		if err != nil {

			s.log.Println("Error projects.index.scan.", err)

			http.Error(w, "Error listing projects.", http.StatusInternalServerError)

			return
		}

		projects = append(projects, project{ID: id, Name: name, Description: description, UserID: user_id, CreatedAt: created_at})
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	tpls.ExecuteTemplate(w, "projects/projects.gohtml", projects)
}

func (s *projectService) store(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := r.Context().Value("user").(user)

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	sql := `
INSERT INTO goissuez.projects
(name, description, user_id, created_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
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
	authUser, _ := r.Context().Value("user").(user)

	project_id := ps.ByName("id")

	r.ParseForm()

	projectName := r.PostForm.Get("name")
	projectDescription := r.PostForm.Get("description")

	stmt, err := s.db.Prepare(`UPDATE goissuez.projects p SET name = $3, description = $4 WHERE p.id = $1 AND p.user_id = $2`)

	if err != nil {
		s.log.Println("Error projects.update.prepare.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(project_id, authUser.ID, projectName, projectDescription)

	if err != nil {
		s.log.Println("Error projects.update.exec.", err)

		http.Error(w, "Error updating project.", http.StatusInternalServerError)

		return
	}

	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *projectService) edit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := r.Context().Value("user").(user)

	project_id := ps.ByName("id")

	stmt, err := s.db.Prepare(`SELECT * FROM goissuez.projects WHERE id = $1 AND user_id = $2 LIMIT 1`)

	if err != nil {
		s.log.Println("Error projects.edit.prepare.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id, authUser.ID)

	var (
		id          int64
		name        string
		description string
		user_id     int64
		created_at  string
	)

	err = row.Scan(&id, &name, &description, &user_id, &created_at)

	if err != nil {
		s.log.Println("Error projects.edit.scan.", err)

		http.Error(w, "Error editing project.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s.tpls.ExecuteTemplate(w, "projects/edit.gohtml", project{ID: id, Name: name, Description: description, UserID: user_id, CreatedAt: created_at})
}

func (s *projectService) show(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := r.Context().Value("user").(user)

	project_id := ps.ByName("id")

	if project_id == "new" {
		s.tpls.ExecuteTemplate(w, "projects/new.gohtml", nil)

		return
	}

	stmt, err := s.db.Prepare(`SELECT * FROM goissuez.projects WHERE id = $1 AND user_id = $2 LIMIT 1`)

	if err != nil {
		s.log.Println("Error projects.show.prepare.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(project_id, authUser.ID)

	var (
		id          int64
		name        string
		description string
		user_id     int64
		created_at  string
	)

	err = row.Scan(&id, &name, &description, &user_id, &created_at)

	if err != nil {
		s.log.Println("Error projects.show.scan.", err)

		http.Error(w, "Error getting project.", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	s.tpls.ExecuteTemplate(w, "projects/project.gohtml", project{ID: id, Name: name, Description: description, UserID: user_id, CreatedAt: created_at})
}

func (s *projectService) destroy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	project_id := ps.ByName("id")

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
