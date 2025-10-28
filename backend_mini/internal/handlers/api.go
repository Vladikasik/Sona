package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"backend_mini/internal/db"
	"backend_mini/internal/util"
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

type eurcTxRequest struct {
	WalletFrom string `json:"wallet_from"`
	WalletTo   string `json:"wallet_to"`
	Amount     string `json:"amount"`
}

type generateMerkleTreeRequest struct {
	OwnerWallet string `json:"owner_wallet"`
}

type mintNFTRequest struct {
	OwnerWallet string `json:"owner_wallet"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Description string `json:"description"`
	SendTo      string `json:"send_to"`
	TreeId      string `json:"tree_id"`
}

type updNFTRequest struct {
	NftAddress string `json:"nft_address"`
	NewStatus  string `json:"new_status"`
	SendTo     string `json:"send_to"`
}

type acceptNFTRequest struct {
	NftAddress    string `json:"nft_address"`
	SenderWallet  string `json:"sender_wallet"`
	PaymentAmount string `json:"payment_amount"`
}

type createChoreRequest struct {
	ParentWallet     string `json:"parent_wallet"`
	ChildWallet      string `json:"child_wallet"`
	ChoreName        string `json:"chore_name"`
	ChoreDescription string `json:"chore_description"`
	BountyAmount     string `json:"bounty_amount"`
}

type updateChoreRequest struct {
	ChoreID   string `json:"chore_id"`
	NewStatus int    `json:"new_status"`
}

type getChoresRequest struct {
	Wallet string `json:"wallet"`
}

type setLimitRequest struct {
	ParentEmail  string `json:"parent_email"`
	KidEmail     string `json:"kid_email"`
	App          string `json:"app"`
	TimePerDay   int    `json:"time_per_day"`
	FeeExtraHour string `json:"fee_extra_hour"`
}

type getLimitsRequest struct {
	KidEmail string `json:"kid_email"`
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

func (a *API) EurcTx(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req eurcTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.WalletFrom) == "" || strings.TrimSpace(req.WalletTo) == "" {
		writeError(w, http.StatusBadRequest, "wallet_from and wallet_to are required")
		return
	}
	amount, err := strconv.ParseUint(req.Amount, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid amount")
		return
	}
	txData, err := util.BuildEURCTransferTransaction(req.WalletFrom, req.WalletTo, amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txData)
}

func (a *API) GenerateMerkleTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req generateMerkleTreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.OwnerWallet) == "" {
		writeError(w, http.StatusBadRequest, "owner_wallet is required")
		return
	}

	result, err := util.CreateMerkleTreeViaJS(req.OwnerWallet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) MintNFT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req mintNFTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.OwnerWallet) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.SendTo) == "" || strings.TrimSpace(req.TreeId) == "" {
		writeError(w, http.StatusBadRequest, "owner_wallet, name, send_to, and tree_id are required")
		return
	}
	txData, err := util.BuildMintNFTTransaction(req.OwnerWallet, req.Name, req.Price, req.Description, req.SendTo, req.TreeId)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txData)
}

func (a *API) UpdNFT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req updNFTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.NftAddress) == "" || strings.TrimSpace(req.NewStatus) == "" || strings.TrimSpace(req.SendTo) == "" {
		writeError(w, http.StatusBadRequest, "nft_address, new_status, and send_to are required")
		return
	}
	txData, err := util.BuildUpdateNFTTransaction(req.NftAddress, req.NewStatus, req.SendTo)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txData)
}

func (a *API) AcceptNFT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req acceptNFTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.NftAddress) == "" || strings.TrimSpace(req.SenderWallet) == "" {
		writeError(w, http.StatusBadRequest, "nft_address and sender_wallet are required")
		return
	}
	paymentAmount, err := strconv.ParseUint(req.PaymentAmount, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment_amount")
		return
	}
	txData, err := util.BuildAcceptNFTTransaction(req.NftAddress, req.SenderWallet, paymentAmount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txData)
}

func (a *API) CreateChore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req createChoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.ParentWallet) == "" || strings.TrimSpace(req.ChildWallet) == "" || strings.TrimSpace(req.ChoreName) == "" {
		writeError(w, http.StatusBadRequest, "parent_wallet, child_wallet, and chore_name are required")
		return
	}
	bountyAmount, err := strconv.ParseUint(req.BountyAmount, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bounty_amount")
		return
	}
	ctx := r.Context()
	chore, err := a.db.CreateChore(ctx, req.ParentWallet, req.ChildWallet, req.ChoreName, req.ChoreDescription, bountyAmount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chore)
}

func (a *API) UpdateChore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req updateChoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.ChoreID) == "" {
		writeError(w, http.StatusBadRequest, "chore_id is required")
		return
	}
	if req.NewStatus < 0 || req.NewStatus > 4 {
		writeError(w, http.StatusBadRequest, "new_status must be between 0 and 4")
		return
	}
	ctx := r.Context()
	chore, err := a.db.UpdateChoreStatus(ctx, req.ChoreID, req.NewStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "chore not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.NewStatus == 3 {
		txData, err := util.BuildEURCTransferTransaction(chore.ParentWallet, chore.ChildWallet, chore.BountyAmount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"chore":       chore,
			"transaction": txData,
		})
		return
	}

	writeJSON(w, http.StatusOK, chore)
}

func (a *API) GetChores(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req getChoresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Wallet) == "" {
		writeError(w, http.StatusBadRequest, "wallet is required")
		return
	}
	ctx := r.Context()
	chores, err := a.db.GetChores(ctx, req.Wallet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chores)
}

func (a *API) SetLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req setLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.ParentEmail) == "" || strings.TrimSpace(req.KidEmail) == "" || strings.TrimSpace(req.App) == "" {
		writeError(w, http.StatusBadRequest, "parent_email, kid_email, and app are required")
		return
	}
	if req.TimePerDay < 0 {
		writeError(w, http.StatusBadRequest, "time_per_day cannot be negative")
		return
	}
	feeExtraHour, err := strconv.ParseUint(req.FeeExtraHour, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fee_extra_hour")
		return
	}
	ctx := r.Context()
	limit, err := a.db.CreateOrUpdateAppLimit(ctx, req.ParentEmail, req.KidEmail, req.App, req.TimePerDay, feeExtraHour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, limit)
}

func (a *API) GetLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req getLimitsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.KidEmail) == "" {
		writeError(w, http.StatusBadRequest, "kid_email is required")
		return
	}
	ctx := r.Context()
	limits, err := a.db.GetAppLimitsByKidEmail(ctx, req.KidEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, limits)
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
