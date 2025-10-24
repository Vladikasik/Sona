package handlers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"sona-backend/internal/db"

	"github.com/google/uuid"
)

type Server struct {
	DB *db.Database
	// Grid configuration
	GridEnv           string
	GridAPIKey        string
	GridBaseURL       string
	GridOTPVerifyURL  string
	GridAccountsURL   string
	GridAuthInitURL   string
	GridAuthVerifyURL string
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
	// Optional email support
	email, hasEmail := parseStringQuery(r, "email")
	var parentID int64
	var parentUID string
	var err error
	if hasEmail && email != "" {
		parentID, parentUID, err = s.DB.CreateParentWithEmail(name, email)
	} else {
		parentID, parentUID, err = s.DB.CreateParent(name)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parent_id": parentID, "parent_uid": parentUID, "email": email})
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
	kidID, _, err := s.DB.CreateKid(name, "", parentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kid_id": kidID})
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
	log.Printf("GET /list_kids parent_id=%d", parentID)
	kids, err := s.DB.ListKids(parentID)
	if err != nil {
		log.Printf("list kids error: %v", err)
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
	email, ok := parseStringQuery(r, "email")
	if !ok || email == "" {
		writeJSON(w, http.StatusBadRequest, apiError{"missing email"})
		return
	}
	log.Printf("GET /get_parent name=%q email=%q", name, email)
	// Ensure a parent record exists and keys are generated
	p, err := s.DB.GetOrCreateParentByEmail(name, email)
	if err != nil {
		log.Printf("get/create parent error: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	if len(p.HPKEPubDER) == 0 || len(p.HPKEPrivDER) == 0 {
		log.Printf("generating HPKE P-256 keys for parent_id=%d", p.ID)
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			log.Printf("key generation failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{"key generation failed"})
			return
		}
		privDER, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			log.Printf("private key DER marshal failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{"private key DER marshal failed"})
			return
		}
		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			log.Printf("public key DER marshal failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{"public key DER marshal failed"})
			return
		}
		if err := s.DB.SetParentHPKEKeys(p.ID, privDER, pubDER); err != nil {
			log.Printf("db store keys failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		p, _ = s.DB.GetParentByID(p.ID)
	}

	// Call Grid Create Account (email type)
	if s.GridAccountsURL == "" || s.GridAPIKey == "" {
		log.Printf("grid not configured: accounts_url=%q api_key_set=%t", s.GridAccountsURL, s.GridAPIKey != "")
		writeJSON(w, http.StatusInternalServerError, apiError{"grid not configured"})
		return
	}
	payload := map[string]any{
		"type":  "email",
		"email": email,
		"memo":  name,
	}
	b, _ := json.Marshal(payload)
	log.Printf("grid create account POST url=%s env=%s", s.GridAccountsURL, s.GridEnv)
	req, err := http.NewRequest(http.MethodPost, s.GridAccountsURL, bytes.NewReader(b))
	if err != nil {
		log.Printf("build request error: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{"failed to build request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
	req.Header.Set("x-grid-environment", s.GridEnv)
	req.Header.Set("x-idempotency-key", uuid.NewString())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("grid request failed: %v", err)
		writeJSON(w, http.StatusBadGateway, apiError{"grid request failed"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("grid create account response status=%d body=%s", resp.StatusCode, truncate(respBody, 1024))

	// If account creation indicates existing user, initiate authentication flow instead
	shouldAuth := false
	if resp.StatusCode == http.StatusConflict || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		shouldAuth = true
	} else if resp.StatusCode >= 400 {
		var msg map[string]any
		if err := json.Unmarshal(respBody, &msg); err == nil {
			m := strings.ToLower(string(respBody))
			if strings.Contains(m, "exists") || strings.Contains(m, "resource_exists") || strings.Contains(m, "already") {
				shouldAuth = true
			}
		}
	}

	if shouldAuth {
		if s.GridAuthInitURL == "" {
			writeJSON(w, http.StatusInternalServerError, apiError{"grid auth not configured"})
			return
		}
		authPayload := map[string]any{
			"email":    email,
			"provider": "privy",
		}
		ab, _ := json.Marshal(authPayload)
		log.Printf("grid auth initiate POST url=%s env=%s", s.GridAuthInitURL, s.GridEnv)
		areq, err := http.NewRequest(http.MethodPost, s.GridAuthInitURL, bytes.NewReader(ab))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"failed to build auth request"})
			return
		}
		areq.Header.Set("Content-Type", "application/json")
		areq.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
		areq.Header.Set("x-grid-environment", s.GridEnv)
		areq.Header.Set("x-idempotency-key", uuid.NewString())
		aresp, err := http.DefaultClient.Do(areq)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, apiError{"grid auth request failed"})
			return
		}
		defer aresp.Body.Close()
		abody, _ := io.ReadAll(aresp.Body)
		log.Printf("grid auth initiate response status=%d body=%s", aresp.StatusCode, truncate(abody, 1024))

		var gridObj any
		_ = json.Unmarshal(abody, &gridObj)
		writeJSON(w, aresp.StatusCode, map[string]any{
			"user_type":       "existing",
			"otp_verify_path": "/grid/auth_otp_verify",
			"grid":            gridObj,
		})
		return
	}

	// New user registration path
	var gridObj any
	_ = json.Unmarshal(respBody, &gridObj)
	writeJSON(w, resp.StatusCode, map[string]any{
		"user_type":       "new",
		"otp_verify_path": "/grid/otp_verify",
		"grid":            gridObj,
	})
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
	log.Printf("GET /get_child name=%q", name)
	if parentID, hasParent := parseInt64Query(r, "parent_id"); hasParent {
		if _, err := s.DB.GetParentByID(parentID); err != nil {
			if err == sql.ErrNoRows {
				log.Printf("get_child parent not found parent_id=%d", parentID)
				writeJSON(w, http.StatusBadRequest, apiError{"parent not found"})
				return
			}
			log.Printf("get_child db error: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		kidID, _, err := s.DB.CreateKid(name, "", parentID)
		if err != nil {
			log.Printf("create kid error: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		k, err := s.DB.GetKidByID(kidID)
		if err != nil {
			log.Printf("get kid by id error: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, k)
		return
	}
	k, err := s.DB.GetKidByName(name)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("get_child not found name=%q", name)
			writeJSON(w, http.StatusOK, nil)
			return
		}
		log.Printf("get_child db error: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, k)
}

// GetUser unifies parent and kid registration/login flows behind a single endpoint.
// Query params:
// - user_type: "parent" | "kid" (or "child")
// - name: required for both
// - email: required for parent
// - parent_id: optional for kid (if provided, will create kid under parent)
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	userType, ok := parseStringQuery(r, "user_type")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing user_type"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}

	switch strings.ToLower(userType) {
	case "parent":
		email, ok := parseStringQuery(r, "email")
		if !ok || email == "" {
			writeJSON(w, http.StatusBadRequest, apiError{"missing email"})
			return
		}
		log.Printf("GET /get_user type=parent name=%q email=%q", name, email)

		// Ensure a parent record exists and keys are generated (same as GetParent)
		p, err := s.DB.GetOrCreateParentByEmail(name, email)
		if err != nil {
			log.Printf("get/create parent error: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		if len(p.HPKEPubDER) == 0 || len(p.HPKEPrivDER) == 0 {
			log.Printf("generating HPKE P-256 keys for parent_id=%d", p.ID)
			priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				log.Printf("key generation failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, apiError{"key generation failed"})
				return
			}
			privDER, err := x509.MarshalECPrivateKey(priv)
			if err != nil {
				log.Printf("private key DER marshal failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, apiError{"private key DER marshal failed"})
				return
			}
			pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
			if err != nil {
				log.Printf("public key DER marshal failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, apiError{"public key DER marshal failed"})
				return
			}
			if err := s.DB.SetParentHPKEKeys(p.ID, privDER, pubDER); err != nil {
				log.Printf("db store keys failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
				return
			}
		}

		// Call Grid Create Account (email type) or fall back to auth if exists (same as GetParent)
		if s.GridAccountsURL == "" || s.GridAPIKey == "" {
			log.Printf("grid not configured: accounts_url=%q api_key_set=%t", s.GridAccountsURL, s.GridAPIKey != "")
			writeJSON(w, http.StatusInternalServerError, apiError{"grid not configured"})
			return
		}
		payload := map[string]any{
			"type":  "email",
			"email": email,
			"memo":  name,
		}
		b, _ := json.Marshal(payload)
		log.Printf("grid create account POST url=%s env=%s", s.GridAccountsURL, s.GridEnv)
		req, err := http.NewRequest(http.MethodPost, s.GridAccountsURL, bytes.NewReader(b))
		if err != nil {
			log.Printf("build request error: %v", err)
			writeJSON(w, http.StatusInternalServerError, apiError{"failed to build request"})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
		req.Header.Set("x-grid-environment", s.GridEnv)
		req.Header.Set("x-idempotency-key", uuid.NewString())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("grid request failed: %v", err)
			writeJSON(w, http.StatusBadGateway, apiError{"grid request failed"})
			return
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("grid create account response status=%d body=%s", resp.StatusCode, truncate(respBody, 1024))

		// If account creation indicates existing user, initiate authentication flow instead
		shouldAuth := false
		if resp.StatusCode == http.StatusConflict || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
			shouldAuth = true
		} else if resp.StatusCode >= 400 {
			var msg map[string]any
			if err := json.Unmarshal(respBody, &msg); err == nil {
				m := strings.ToLower(string(respBody))
				if strings.Contains(m, "exists") || strings.Contains(m, "resource_exists") || strings.Contains(m, "already") {
					shouldAuth = true
				}
			}
		}

		if shouldAuth {
			if s.GridAuthInitURL == "" {
				writeJSON(w, http.StatusInternalServerError, apiError{"grid auth not configured"})
				return
			}
			authPayload := map[string]any{
				"email":    email,
				"provider": "privy",
			}
			ab, _ := json.Marshal(authPayload)
			log.Printf("grid auth initiate POST url=%s env=%s", s.GridAuthInitURL, s.GridEnv)
			areq, err := http.NewRequest(http.MethodPost, s.GridAuthInitURL, bytes.NewReader(ab))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"failed to build auth request"})
				return
			}
			areq.Header.Set("Content-Type", "application/json")
			areq.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
			areq.Header.Set("x-grid-environment", s.GridEnv)
			areq.Header.Set("x-idempotency-key", uuid.NewString())
			aresp, err := http.DefaultClient.Do(areq)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, apiError{"grid auth request failed"})
				return
			}
			defer aresp.Body.Close()
			abody, _ := io.ReadAll(aresp.Body)
			log.Printf("grid auth initiate response status=%d body=%s", aresp.StatusCode, truncate(abody, 1024))

			var gridObj any
			_ = json.Unmarshal(abody, &gridObj)
			writeJSON(w, aresp.StatusCode, map[string]any{
				"user_type":       "existing",
				"otp_verify_path": "/grid/auth_otp_verify",
				"grid":            gridObj,
			})
			return
		}

		// New user registration path
		var gridObj any
		_ = json.Unmarshal(respBody, &gridObj)
		writeJSON(w, resp.StatusCode, map[string]any{
			"user_type":       "new",
			"otp_verify_path": "/grid/otp_verify",
			"grid":            gridObj,
		})
		return

	case "kid", "child":
		// For kids, require parent_id and email; mirror parent Grid flow
		email, ok := parseStringQuery(r, "email")
		if !ok || email == "" {
			writeJSON(w, http.StatusBadRequest, apiError{"missing email"})
			return
		}
		parentID, ok := parseInt64Query(r, "parent_id")
		if !ok {
			writeJSON(w, http.StatusBadRequest, apiError{"missing parent_id"})
			return
		}
		if _, err := s.DB.GetParentByID(parentID); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, http.StatusBadRequest, apiError{"parent not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}

		// Ensure kid record exists (by name+parent) or create; set email if empty
		k, err := s.DB.GetKidByNameAndParent(name, parentID)
		if err == sql.ErrNoRows {
			_, _, errCreate := s.DB.CreateKid(name, email, parentID)
			if errCreate != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{errCreate.Error()})
				return
			}
			k, err = s.DB.GetKidByNameAndParent(name, parentID)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		if k.Email == "" {
			// set email on first time
			// reuse UpdateKidGridIDs to avoid new method; use separate update
			_, err := s.DB.DB.Exec(`UPDATE kids SET email = ? WHERE id = ?`, email, k.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
				return
			}
			k.Email = email
		}

		// Ensure HPKE keys exist for kid
		if len(k.HPKEPubDER) == 0 || len(k.HPKEPrivDER) == 0 {
			priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"key generation failed"})
				return
			}
			privDER, err := x509.MarshalECPrivateKey(priv)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"private key DER marshal failed"})
				return
			}
			pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"public key DER marshal failed"})
				return
			}
			if err := s.DB.SetKidHPKEKeys(k.ID, privDER, pubDER); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
				return
			}
			k, _ = s.DB.GetKidByID(k.ID)
		}

		// Mirror Grid flow using kid email
		if s.GridAccountsURL == "" || s.GridAPIKey == "" {
			writeJSON(w, http.StatusInternalServerError, apiError{"grid not configured"})
			return
		}
		payload := map[string]any{
			"type":  "email",
			"email": email,
			"memo":  name,
		}
		b, _ := json.Marshal(payload)
		req, err := http.NewRequest(http.MethodPost, s.GridAccountsURL, bytes.NewReader(b))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"failed to build request"})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
		req.Header.Set("x-grid-environment", s.GridEnv)
		req.Header.Set("x-idempotency-key", uuid.NewString())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, apiError{"grid request failed"})
			return
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		shouldAuth := false
		if resp.StatusCode == http.StatusConflict || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
			shouldAuth = true
		} else if resp.StatusCode >= 400 {
			m := strings.ToLower(string(respBody))
			if strings.Contains(m, "exists") || strings.Contains(m, "resource_exists") || strings.Contains(m, "already") {
				shouldAuth = true
			}
		}

		if shouldAuth {
			if s.GridAuthInitURL == "" {
				writeJSON(w, http.StatusInternalServerError, apiError{"grid auth not configured"})
				return
			}
			authPayload := map[string]any{
				"email":    email,
				"provider": "privy",
			}
			ab, _ := json.Marshal(authPayload)
			areq, err := http.NewRequest(http.MethodPost, s.GridAuthInitURL, bytes.NewReader(ab))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{"failed to build auth request"})
				return
			}
			areq.Header.Set("Content-Type", "application/json")
			areq.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
			areq.Header.Set("x-grid-environment", s.GridEnv)
			areq.Header.Set("x-idempotency-key", uuid.NewString())
			aresp, err := http.DefaultClient.Do(areq)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, apiError{"grid auth request failed"})
				return
			}
			defer aresp.Body.Close()
			abody, _ := io.ReadAll(aresp.Body)

			var gridObj any
			_ = json.Unmarshal(abody, &gridObj)
			writeJSON(w, aresp.StatusCode, map[string]any{
				"user_type":       "existing",
				"otp_verify_path": "/grid/auth_otp_verify",
				"grid":            gridObj,
			})
			return
		}

		var gridObj any
		_ = json.Unmarshal(respBody, &gridObj)
		writeJSON(w, resp.StatusCode, map[string]any{
			"user_type":       "new",
			"otp_verify_path": "/grid/otp_verify",
			"grid":            gridObj,
		})
		return

	default:
		writeJSON(w, http.StatusBadRequest, apiError{"invalid user_type"})
		return
	}
}

// RegisterGridAccount registers a parent with email, generates HPKE P-256 keys in DER, stores them, and returns public key for Grid.
func (s *Server) RegisterGridAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	name, ok := parseStringQuery(r, "name")
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{"missing name"})
		return
	}
	email, ok := parseStringQuery(r, "email")
	if !ok || email == "" {
		writeJSON(w, http.StatusBadRequest, apiError{"missing email"})
		return
	}
	// Create or get parent by email
	p, err := s.DB.GetOrCreateParentByEmail(name, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
	// If keys already exist, reuse
	if len(p.HPKEPubDER) == 0 || len(p.HPKEPrivDER) == 0 {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"key generation failed"})
			return
		}
		privDER, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"private key DER marshal failed"})
			return
		}
		pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{"public key DER marshal failed"})
			return
		}
		if err := s.DB.SetParentHPKEKeys(p.ID, privDER, pubDER); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
			return
		}
		// refresh parent
		p, _ = s.DB.GetParentByID(p.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"parent_id":               p.ID,
		"parent_uid":              p.UID,
		"email":                   p.Email,
		"grid_env":                s.GridEnv,
		"hpke_public_key_der_b64": base64.StdEncoding.EncodeToString(p.HPKEPubDER),
	})
}

// VerifyGridOTP proxies OTP verification to the Grid endpoint with required headers.
func (s *Server) VerifyGridOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	if s.GridOTPVerifyURL == "" || s.GridAPIKey == "" {
		log.Printf("grid verification not configured: url=%q api_key_set=%t", s.GridOTPVerifyURL, s.GridAPIKey != "")
		writeJSON(w, http.StatusInternalServerError, apiError{"grid verification not configured"})
		return
	}
	type verifyReq struct {
		Email string `json:"email"`
		OTP   string `json:"otp_code"`
	}
	var vr verifyReq
	if err := json.NewDecoder(r.Body).Decode(&vr); err != nil {
		log.Printf("/grid/otp_verify invalid json: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{"invalid json"})
		return
	}
	if vr.Email == "" || vr.OTP == "" {
		log.Printf("/grid/otp_verify missing fields email=%q otp_len=%d", vr.Email, len(vr.OTP))
		writeJSON(w, http.StatusBadRequest, apiError{"email and otp_code required"})
		return
	}
	log.Printf("POST /grid/otp_verify email=%q", vr.Email)
	p, err := s.DB.GetParentByEmail(vr.Email)
	if err != nil {
		log.Printf("parent not found by email: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{"parent not found"})
		return
	}
	// ensure keys exist
	if len(p.HPKEPubDER) == 0 {
		log.Printf("missing public key for parent_id=%d", p.ID)
		writeJSON(w, http.StatusInternalServerError, apiError{"public key missing"})
		return
	}
	payload := map[string]any{
		"email":    vr.Email,
		"otp_code": vr.OTP,
		"kms_provider_config": map[string]any{
			"encryption_public_key": base64.StdEncoding.EncodeToString(p.HPKEPubDER),
		},
	}
	b, _ := json.Marshal(payload)
	log.Printf("grid verify OTP POST url=%s env=%s", s.GridOTPVerifyURL, s.GridEnv)
	req, err := http.NewRequest(http.MethodPost, s.GridOTPVerifyURL, bytes.NewReader(b))
	if err != nil {
		log.Printf("build request error: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{"failed to build request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
	req.Header.Set("x-grid-environment", s.GridEnv)
	req.Header.Set("x-idempotency-key", uuid.NewString())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("grid request failed: %v", err)
		writeJSON(w, http.StatusBadGateway, apiError{"grid request failed"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("grid verify response status=%d body=%s", resp.StatusCode, truncate(respBody, 1024))

	// Attempt to extract grid_user_id and address to store
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		if data, ok := parsed["data"].(map[string]any); ok {
			var userID, address string
			if v, ok := data["grid_user_id"].(string); ok {
				userID = v
			}
			if v, ok := data["address"].(string); ok {
				address = v
			}
			if userID != "" || address != "" {
				log.Printf("storing grid ids for parent_id=%d user_id=%q address=%q", p.ID, userID, address)
				_ = s.DB.UpdateParentGridIDs(p.ID, userID, address)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// VerifyGridAuthOTP proxies OTP verification for existing Grid accounts (authentication flow).
func (s *Server) VerifyGridAuthOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"method not allowed"})
		return
	}
	if s.GridAuthVerifyURL == "" || s.GridAPIKey == "" {
		log.Printf("grid auth verification not configured: url=%q api_key_set=%t", s.GridAuthVerifyURL, s.GridAPIKey != "")
		writeJSON(w, http.StatusInternalServerError, apiError{"grid auth verification not configured"})
		return
	}
	type verifyReq struct {
		Email string `json:"email"`
		OTP   string `json:"otp_code"`
	}
	var vr verifyReq
	if err := json.NewDecoder(r.Body).Decode(&vr); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"invalid json"})
		return
	}
	if vr.Email == "" || vr.OTP == "" {
		writeJSON(w, http.StatusBadRequest, apiError{"email and otp_code required"})
		return
	}
	p, err := s.DB.GetParentByEmail(vr.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"parent not found"})
		return
	}
	if len(p.HPKEPubDER) == 0 {
		writeJSON(w, http.StatusInternalServerError, apiError{"public key missing"})
		return
	}
	payload := map[string]any{
		"email":        vr.Email,
		"otp_code":     vr.OTP,
		"kms_provider": "privy",
		"kms_provider_config": map[string]any{
			"encryption_public_key": base64.StdEncoding.EncodeToString(p.HPKEPubDER),
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, s.GridAuthVerifyURL, bytes.NewReader(b))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{"failed to build request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.GridAPIKey)
	req.Header.Set("x-grid-environment", s.GridEnv)
	req.Header.Set("x-idempotency-key", uuid.NewString())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, apiError{"grid request failed"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	// Store grid ids if present
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		if data, ok := parsed["data"].(map[string]any); ok {
			var userID, address string
			if v, ok := data["grid_user_id"].(string); ok {
				userID = v
			}
			if v, ok := data["address"].(string); ok {
				address = v
			}
			if userID != "" || address != "" {
				_ = s.DB.UpdateParentGridIDs(p.ID, userID, address)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// truncate limits logged body to n bytes
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "... (truncated)"
}
