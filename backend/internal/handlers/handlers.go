package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"sona-backend/internal/db"
)

type Server struct {
	DB *db.Database
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseInt64Query(r *http.Request, key string) (int64, bool) {
	val := r.URL.Query().Get(key)
	if val == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseStringQuery(r *http.Request, key string) (string, bool) {
	val := r.URL.Query().Get(key)
	if val == "" {
		return "", false
	}
	return val, true
}

func (s *Server) RegisterParent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}
	parentID, parentUID, err := s.DB.CreateParent(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parent_id": parentID, "parent_uid": parentUID})
}

func (s *Server) RegisterKid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}
	parentID, ok := parseInt64Query(r, "parent_id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing parent_id"})
		return
	}
	id, err := s.DB.CreateKid(name, parentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kid_id": id})
}

func (s *Server) KidsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	parentID, ok := parseInt64Query(r, "parent_id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing parent_id"})
		return
	}
	kids, err := s.DB.ListKids(parentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kids": kids})
}

func (s *Server) ParentTopUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	parentID, ok := parseInt64Query(r, "parent_id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing parent_id"})
		return
	}
	amount, ok := parseInt64Query(r, "amount")
	if !ok || amount <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{"invalid amount"})
		return
	}
	bal, err := s.DB.TopUpParent(parentID, amount)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parent_balance": bal})
}

func (s *Server) SendKidMoney(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	parentID, ok := parseInt64Query(r, "parent_id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing parent_id"})
		return
	}
	kidID, ok := parseInt64Query(r, "kid_id")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing kid_id"})
		return
	}
	amount, ok := parseInt64Query(r, "amount")
	if !ok || amount <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{"invalid amount"})
		return
	}
	parentBal, kidBal, err := s.DB.SendKidMoney(parentID, kidID, amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parent_balance": parentBal, "kid_balance": kidBal})
}

func (s *Server) GetParent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}
	p, err := s.DB.GetOrCreateParentByName(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) GetChild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}
	if parentID, hasParent := parseInt64Query(r, "parent_id"); hasParent {
		if _, err := s.DB.GetParentByID(parentID); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, http.StatusBadRequest, apiError{"parent not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		kidID, err := s.DB.CreateKid(name, parentID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		k, err := s.DB.GetKidByID(kidID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, k)
		return
	}
	k, err := s.DB.GetKidByName(name)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusOK, nil)
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, k)
}
