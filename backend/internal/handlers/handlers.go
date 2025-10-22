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
	"net/http"
	"strconv"

	"sona-backend/internal/db"

	"github.com/google/uuid"
)

type Server struct {
	DB *db.Database
	// Grid configuration
	GridEnv          string
	GridAPIKey       string
	GridBaseURL      string
	GridOTPVerifyURL string
	GridAccountsURL  string
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
	email, ok := parseStringQuery(r, "email")
	if !ok || email == "" {
		writeJSON(w, http.StatusBadRequest, apiError{"missing email"})
		return
	}
	// Ensure a parent record exists and keys are generated
	p, err := s.DB.GetOrCreateParentByEmail(name, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{err.Error()})
		return
	}
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
		p, _ = s.DB.GetParentByID(p.ID)
	}

	// Call Grid Create Account (email type)
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
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
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
		writeJSON(w, http.StatusInternalServerError, apiError{"grid verification not configured"})
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
	// ensure keys exist
	if len(p.HPKEPubDER) == 0 {
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
	req, err := http.NewRequest(http.MethodPost, s.GridOTPVerifyURL, bytes.NewReader(b))
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
				_ = s.DB.UpdateParentGridIDs(p.ID, userID, address)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}
