package handlers

import (
	"encoding/json"
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
	depth := uint8(15)
	maxBufferSize := uint8(64)
	txData, err := util.BuildMerkleTreeTransaction(req.OwnerWallet, depth, maxBufferSize)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txData)
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
