package main

import (
	"context"
	"database/sql"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strconv"

	"encoding/json"
	"github.com/julienschmidt/httprouter"
)

type adminService struct {
	db   *sql.DB
	log  *logrus.Logger
	tpls *template.Template
}

type capability struct {
	ID          int64
	Name        string
	Description string
	Group       string
}

type role struct {
	ID           int64
	Name         string
	Description  string
	Capabilities []*capability
}

func NewAdminService(db *sql.DB, logger *logrus.Logger, tpls *template.Template) *adminService {
	return &adminService{db, logger, tpls}
}

func (s *adminService) index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"admin"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pageData := page{Title: "Admin Panel - Welcome, " + authUser.Name + "!"}

	view := NewView(w, r)

	view.make("templates/admin/index.gohtml")

	err := view.exec(mainLayout, pageData)

	if err != nil {
		http.Error(w, "Error loading admin panel", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *adminService) users(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"admin_read_users"}) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	users := []user{}
	roles, err := s.getRoles()

	if err != nil {
		s.log.Error("Error admin.users.getroles", err)
	}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
username,
photo_url,
role_id,
deleted_at
FROM goissuez.users
`)

	if err != nil {
		s.log.Error("Error admin.users.perpare", err)

		return
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		s.log.Error("Error admin.users.query", err)

		return
	}

	for rows.Next() {

		userData := user{}
		var photo_url sql.NullString
		var role_id sql.NullInt64
		var deleted_at sql.NullString

		err := rows.Scan(
			&userData.ID,
			&userData.Name,
			&userData.Username,
			&photo_url,
			&role_id,
			&deleted_at,
		)

		if err != nil {
			s.log.Error("Error admin.users.scan", err)

			return
		}

		if deleted_at.Valid {
			userData.DeletedAt = deleted_at.String
		}

		if photo_url.Valid {
			userData.PhotoUrl = photo_url.String
		}

		if role_id.Valid {
			userData.RoleID = role_id.Int64

			for _, role := range roles {
				if role.ID == userData.RoleID {
					userData.Role = role
				}
			}
		}

		users = append(users, userData)
	}

	pageData := page{
		Title: "Users",
		Data: struct {
			Users []user
			Roles []role
		}{users, roles},
		Funcs: make(map[string]interface{}),
	}

	pageData.Funcs["ToJSON"] = func(userData user) string {
		b, err := json.Marshal(userData)

		if err != nil {
			return ""
		}

		return string(b)
	}

	view := viewService{w: w, r: r}
	view.make("templates/admin/users.gohtml")

	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error("Error admin.users.view.exec", err)

		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	view.send(http.StatusOK)

}

func (s *adminService) roles(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"read_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	roles := []role{}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
description
FROM goissuez.roles
ORDER BY name
`)

	if err != nil {
		s.log.Error("Error admin.roles.prepare.roles", err)
		return
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		s.log.Error("Error admin.roles.query.roles", err)
		return
	}

	for rows.Next() {

		roleData := role{}

		description := sql.NullString{}

		err := rows.Scan(
			&roleData.ID,
			&roleData.Name,
			&description,
		)

		if err != nil {
			s.log.Error("Error admin.roles.scan.roles", err)
			return
		}

		if description.Valid {
			roleData.Description = description.String
		}

		roles = append(roles, roleData)
	}

	capabilities, _ := s.getCapabilities()

	pageData := page{
		Title: "Roles",
		Data: struct {
			Roles        []role
			Capabilities []*capability
		}{
			roles,
			capabilities,
		},
		Funcs: make(map[string]interface{}),
	}

	pageData.Funcs["ToJSON"] = func(roleData role) string {
		b, err := json.Marshal(roleData)

		if err != nil {
			return ""
		}

		return string(b)
	}

	view := viewService{w: w, r: r}

	view.make("templates/admin/roles.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error("Error admin.roles.exec", err)
		http.Error(w, "Error loading roles.", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)

}

func (s *adminService) role(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"read_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	roleData := role{Capabilities: []*capability{}}
	role_id := ps.ByName("role_id")

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
description
FROM goissuez.roles
WHERE id = $1
LIMIT 1
`)

	if err != nil {
		s.log.Error("Error admin.show.prepare.role", err)

		http.Error(w, "Error loading role.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(role_id)

	description := sql.NullString{}

	err = row.Scan(
		&roleData.ID,
		&roleData.Name,
		&description,
	)

	if err != nil {
		s.log.Error("Error admin.show.scan.role", err)

		http.Error(w, "Error loading role.", http.StatusInternalServerError)
		return
	}

	if description.Valid {
		roleData.Description = description.String
	}

	capabilities, err := s.getCapabilities()

	if err != nil {
		s.log.Error("Error admin.show.role.getcapabilities", err)
		http.Error(w, "Error loading role.", http.StatusInternalServerError)
		return
	}

	roleData.Capabilities = capabilities

	roleID, _ := strconv.ParseInt(role_id, 10, 64)
	permissions, err := s.getRolePermissions(roleID)

	if err != nil {
		s.log.Error("Error admin.show.role.getrolepermissions", err)
		http.Error(w, "Error loading role.", http.StatusInternalServerError)
		return
	}

	pageData := page{
		Title: "Role : " + roleData.Name,
		Data: struct {
			Role        role
			Permissions map[string]capability
		}{
			roleData,
			permissions,
		},
	}

	view := viewService{w: w, r: r}

	view.make("templates/admin/role.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error("Error admin.role.exec", err)
		http.Error(w, "Error loading role.", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)
}

func (s *adminService) demo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	roles := map[string]string{"admin": "admin_demo", "manager": "manager_demo", "qa": "qa_demo", "developer": "developer_demo"}
	role := ps.ByName("role")

	username, ok := roles[role]

	if !ok {
		return
	}

	userData, err := getUserByUsername(s.db, username)

	if err != nil {
		s.log.Error("Error admin.demo.getuserbyusername."+role, err)
		return
	}

	// don't bother verifying the password
	err = auth.authenticateUser(userData.ID, w)

	if err != nil {
		s.log.Error("Error admin.demo.authenticateuser."+role, err)
		return
	}

	ctx := context.WithValue(r.Context(), "user", userData)

	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusSeeOther)
}

// savePermissions saves an array of capability ids for a given role_id.
// Each capability is saved separately.
func (s *adminService) savePermissions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"update_permissions"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	role_id := ps.ByName("role_id")

	r.ParseForm()

	permissions := r.Form["permissions"]

	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)

	if err != nil {
		s.log.Error("Error admin.savePermissiones.begintx", err)
		return
	}

	stmt, err := tx.Prepare(`
DELETE FROM goissuez.permissions
WHERE role_id = $1
`)

	if err != nil {
		s.log.Error("Error admin.savePermissiones.delete.prepare.", err)
		tx.Rollback()
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(role_id)

	if err != nil {
		s.log.Error("Error admin.savePermissiones.delete.exec", err)
		tx.Rollback()
		return
	}

	insertQuery := `
	  INSERT INTO goissuez.permissions
	  (role_id, capability_id) VALUES
	`

	// stmt.Exec requires []interface{}{}
	vals := []interface{}{}

	paramCount := 1
	for i, id := range permissions {
		if i < len(permissions)-1 {
			insertQuery += `($` + strconv.Itoa(paramCount) + `, $` + strconv.Itoa(paramCount+1) + `),`
		} else {
			insertQuery += `($` + strconv.Itoa(paramCount) + `, $` + strconv.Itoa(paramCount+1) + `)`
		}

		vals = append(vals, role_id, id)
		paramCount += 2
	}

	stmt, err = tx.Prepare(insertQuery)

	if err != nil {
		s.log.Error("Error admin.savePermissiones.insert.prepare", err)
		tx.Rollback()
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(vals...)

	if err != nil {
		s.log.Error("Error admin.savePermissiones.insert.exec", err)
		tx.Rollback()
		return
	}

	err = tx.Commit()

	if err != nil {
		s.log.Error("Error admin.savePermissiones.delete.exec", err)
		return
	}

	http.Redirect(w, r, "/roles/"+role_id, http.StatusSeeOther)
}

func (s *adminService) getCapabilities() ([]*capability, error) {
	capabilities := []*capability{}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
description,
"group"
FROM goissuez.capabilities
`)

	if err != nil {
		return capabilities, err
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		return capabilities, err
	}

	for rows.Next() {
		data := capability{}

		description := sql.NullString{}
		err := rows.Scan(
			&data.ID,
			&data.Name,
			&description,
			&data.Group,
		)

		if err != nil {
			return capabilities, err
		}

		if description.Valid {
			data.Description = description.String
		}

		capabilities = append(capabilities, &data)
	}

	return capabilities, nil
}

func (s *adminService) getRoles() ([]role, error) {
	roles := []role{}

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
description
FROM goissuez.roles
`)

	if err != nil {
		return roles, err
	}

	defer stmt.Close()

	rows, err := stmt.Query()

	if err != nil {
		return roles, err
	}

	for rows.Next() {

		roleData := role{}

		err := rows.Scan(
			&roleData.ID,
			&roleData.Name,
			&roleData.Description,
		)

		if err != nil {
			return roles, err
		}

		roles = append(roles, roleData)
	}

	return roles, nil
}

func (s *adminService) getRolePermissions(role_id int64) (map[string]capability, error) {
	permissions := make(map[string]capability)

	stmt, err := s.db.Prepare(`
SELECT
p.capability_id,
c.name,
c.description,
c.group
FROM goissuez.permissions p
JOIN goissuez.capabilities c
ON c.id = p.capability_id
WHERE role_id = $1
`)

	if err != nil {
		return permissions, err
	}

	defer stmt.Close()

	rows, err := stmt.Query(role_id)

	if err != nil {
		return permissions, err
	}

	for rows.Next() {
		capData := capability{}

		description := sql.NullString{}

		err := rows.Scan(
			&capData.ID,
			&capData.Name,
			&description,
			&capData.Group,
		)

		if err != nil {
			return permissions, err
		}

		if description.Valid {
			capData.Description = description.String
		}

		permissions[capData.Name] = capData
	}

	return permissions, nil
}

func (s *adminService) storeRole(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"create_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseForm()

	name := r.PostFormValue("name")
	description := r.PostFormValue("description")

	stmt, err := s.db.Prepare(`
INSERT INTO goissuez.roles
(name, description)
VALUES ($1, $2)
RETURNING id
`)

	if err != nil {
		s.log.Error("Error admin.createrole.prepare.", err)

		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(name, description)

	if err != nil {
		s.log.Error("Error admin.createrole.query.", err)

		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/roles", http.StatusSeeOther)
}

func (s *adminService) updateRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"update_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	role_id := ps.ByName("role_id")

	r.ParseForm()

	name := r.PostFormValue("name")
	description := r.PostFormValue("description")

	stmt, err := s.db.Prepare(`
UPDATE goissuez.roles
SET name = $2,
description = $3
WHERE id = $1
`)

	if err != nil {
		s.log.Error("Error admin.updaterole.sql.prepare. ", err)

		http.Error(w, "Error updating role.", http.StatusInternalServerError)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(role_id, name, description)

	if err != nil {
		s.log.Error("Error admin.updaterole.sql.queryrow. ", err)

		http.Error(w, "Error updating role.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/roles", http.StatusSeeOther)
}

func (s *adminService) createRole(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"create_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pageData := page{Title: "Create a Role"}

	view := NewView(w, r)

	view.make("templates/admin/newrole.gohtml")

	err := view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error("Error admin.createrole.view.exec", err)

		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	view.send(http.StatusOK)
}

func (s *adminService) editRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"update_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	role_id := ps.ByName("role_id")

	stmt, err := s.db.Prepare(`
SELECT
id,
name,
description
FROM goissuez.roles
WHERE id = $1
LIMIT 1
`)

	if err != nil {
		s.log.Error("Error admin.editrole.prepare.", err)

		http.Error(w, "Error loading role.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	row := stmt.QueryRow(role_id)

	roleData := role{}
	description := sql.NullString{}

	err = row.Scan(
		&roleData.ID,
		&roleData.Name,
		&description,
	)

	if err != nil {
		s.log.Error("Error admin.editrole.scan.", err)

		http.Error(w, "Error loading role.", http.StatusInternalServerError)
		return
	}

	if description.Valid {
		roleData.Description = description.String
	}

	pageData := page{
		Title: "Edit Role : " + roleData.Name,
		Data: struct {
			Role role
		}{
			roleData,
		},
	}

	view := viewService{w: w, r: r}

	view.make("templates/admin/editrole.gohtml")
	err = view.exec(mainLayout, pageData)

	if err != nil {
		s.log.Error("Error admin.editrole.view.exec", err)
		http.Error(w, "Error loading role.", http.StatusInternalServerError)

		return
	}

	view.send(http.StatusOK)

}

func (s *adminService) destroyRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"delete_role"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	role_id := ps.ByName("role_id")

	stmt, err := s.db.Prepare(`DELETE from goissuez.roles WHERE id = $1`)

	if err != nil {
		s.log.Error("Error roles.destroy.prepare.", err)

		http.Error(w, "Error deleting role.", http.StatusInternalServerError)

		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(role_id)

	if err != nil {
		s.log.Error("Error role.destroy.exec.", err)

		http.Error(w, "Error deleting role.", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}

func (s *adminService) setUserRole(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	authUser, _ := auth.getAuthUser(r)

	if !authUser.IsAdmin && !authUser.Can([]string{"admin_update_users"}) {

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var request struct {
		UserID string `json:"user_id"`
		RoleID string `json:"role_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)

	if err != nil {
		s.log.Error("Error admin.setuserrole.decodejson", err)

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	stmt, err := s.db.Prepare(`
UPDATE goissuez.users
SET role_id = $2
WHERE id = $1
`)

	if err != nil {
		s.log.Error("Error admin.setuserrole.prepare", err)

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	defer stmt.Close()

	_, err = stmt.Exec(request.UserID, request.RoleID)

	if err != nil {
		s.log.Error("Error admin.setuserrole.exec", err)

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	w.WriteHeader(http.StatusOK)
}
