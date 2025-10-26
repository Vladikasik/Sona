package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend_mini/internal/db"
)

type API struct {
	db *db.DB
}

func NewAPI(d *db.DB) *API { return &API{db: d} }

type parentRequest struct {
	Email  string  `json:"email"`
	Name   *string `json:"name,omitempty"`
	Wallet *string `json:"wallet,omitempty"`
	Upd    bool    `json:"upd,omitempty"`
}

type childRequest struct {
	Email    string  `json:"email"`
	Name     *string `json:"name,omitempty"`
	ParentID *string `json:"parent_id,omitempty"`
	Wallet   *string `json:"wallet,omitempty"`
	Upd      bool    `json:"upd,omitempty"`
}

func (a *API) GetParent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req parentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	ctx := r.Context()
	if p, found, err := a.db.GetParentByEmail(ctx, req.Email); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if found {
		if req.Upd {
			updated, err := a.db.UpdateParentByEmail(ctx, req.Email, req.Name, req.Wallet)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		writeJSON(w, http.StatusOK, p)
		return
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		created, err := a.db.CreateParent(ctx, *req.Name, req.Email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, created)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func (a *API) GetChild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req childRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	ctx := r.Context()
	if c, found, err := a.db.GetChildByEmail(ctx, req.Email); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if found {
		if req.Upd {
			updated, err := a.db.UpdateChildByEmail(ctx, req.Email, req.Name, req.ParentID, req.Wallet)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		}
		writeJSON(w, http.StatusOK, c)
		return
	}

	if req.Name != nil && req.ParentID != nil && strings.TrimSpace(*req.Name) != "" && strings.TrimSpace(*req.ParentID) != "" {
		created, err := a.db.CreateChild(ctx, *req.Name, req.Email, *req.ParentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, created)
		return
	}
	writeError(w, http.StatusBadRequest, "for user creation you need all: email, name, parent_id")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
