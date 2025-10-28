package util

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const (
	EURCMintDevnet         = "HzwqbKZw8HxMN6bF2yFZNrht3c2iXXzpKcFu7uBEDKtr"
	EURCDecimals           = 6
	TokenProgram           = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	AssociatedTokenProgram = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
	BubblegumProgram       = "BGUMAp9Gq7iTEuizy4pqaxsTyUCbc68BEFgBMRrLFVo"
)

type TransactionData struct {
	Serialized         string            `json:"serialized"`
	Instructions       []InstructionData `json:"instructions"`
	RecentBlockhash    string            `json:"recent_blockhash"`
	FeePayer           string            `json:"fee_payer"`
	RequiredSignatures []string          `json:"required_signatures"`
}

type InstructionData struct {
	ProgramID       string        `json:"program_id"`
	Accounts        []AccountMeta `json:"accounts"`
	Data            string        `json:"data"`
	InstructionType string        `json:"instruction_type"`
}

type AccountMeta struct {
	Pubkey     string `json:"pubkey"`
	IsSigner   bool   `json:"is_signer"`
	IsWritable bool   `json:"is_writable"`
	IsPayer    bool   `json:"is_payer"`
}

type simpleInstruction struct {
	programID solana.PublicKey
	accounts  solana.AccountMetaSlice
	data      []byte
}

func (s *simpleInstruction) ProgramID() solana.PublicKey { return s.programID }
func (s *simpleInstruction) Accounts() []*solana.AccountMeta { return s.accounts }
func (s *simpleInstruction) Data() ([]byte, error) { return s.data, nil }

func BuildEURCTransferTransaction(from, to string, amount uint64) (*TransactionData, error) {
	fromPubkey, err := solana.PublicKeyFromBase58(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}

	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	eurcMint, err := solana.PublicKeyFromBase58(EURCMintDevnet)
	if err != nil {
		return nil, fmt.Errorf("invalid EURC mint: %w", err)
	}

	tokenProgramID, err := solana.PublicKeyFromBase58(TokenProgram)
	if err != nil {
		return nil, fmt.Errorf("invalid token program: %w", err)
	}

	ataProgramID, err := solana.PublicKeyFromBase58(AssociatedTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("invalid ATA program: %w", err)
	}

	fromATA, _, err := solana.FindProgramAddress(
		[][]byte{
			fromPubkey.Bytes(),
			tokenProgramID.Bytes(),
			eurcMint.Bytes(),
		},
		ataProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive from ATA: %w", err)
	}

	toATA, _, err := solana.FindProgramAddress(
		[][]byte{
			toPubkey.Bytes(),
			tokenProgramID.Bytes(),
			eurcMint.Bytes(),
		},
		ataProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive to ATA: %w", err)
	}

	systemProgramID := solana.SystemProgramID

	var instructions []InstructionData

	instructions = append(instructions, InstructionData{
		ProgramID:       AssociatedTokenProgram,
		InstructionType: "create_associated_token_account",
		Accounts: []AccountMeta{
			{Pubkey: fromPubkey.String(), IsSigner: true, IsWritable: true, IsPayer: true},
			{Pubkey: toATA.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: toPubkey.String(), IsSigner: false, IsWritable: false, IsPayer: false},
			{Pubkey: eurcMint.String(), IsSigner: false, IsWritable: false, IsPayer: false},
			{Pubkey: systemProgramID.String(), IsSigner: false, IsWritable: false, IsPayer: false},
			{Pubkey: tokenProgramID.String(), IsSigner: false, IsWritable: false, IsPayer: false},
		},
		Data: "",
	})

	instructions = append(instructions, InstructionData{
		ProgramID:       TokenProgram,
		InstructionType: "transfer_checked",
		Accounts: []AccountMeta{
			{Pubkey: fromATA.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: eurcMint.String(), IsSigner: false, IsWritable: false, IsPayer: false},
			{Pubkey: toATA.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: fromPubkey.String(), IsSigner: true, IsWritable: false, IsPayer: true},
		},
		Data: fmt.Sprintf("%x", amount),
	})

	binaryData := make([]byte, 10)
	binaryData[0] = 12
	binary.LittleEndian.PutUint64(binaryData[1:9], amount)
	binaryData[9] = EURCDecimals

	dummyBlockhash := solana.MustHashFromBase58("11111111111111111111111111111111")

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			&simpleInstruction{
				programID: ataProgramID,
				accounts: solana.AccountMetaSlice{
					{solana.MustPublicKeyFromBase58(fromPubkey.String()), true, true},
					{solana.MustPublicKeyFromBase58(toATA.String()), false, true},
					{solana.MustPublicKeyFromBase58(toPubkey.String()), false, false},
					{solana.MustPublicKeyFromBase58(eurcMint.String()), false, false},
					{solana.SystemProgramID, false, false},
					{solana.MustPublicKeyFromBase58(tokenProgramID.String()), false, false},
				},
				data: []byte{0},
			},
			&simpleInstruction{
				programID: tokenProgramID,
				accounts: solana.AccountMetaSlice{
					{solana.MustPublicKeyFromBase58(fromATA.String()), false, true},
					{solana.MustPublicKeyFromBase58(eurcMint.String()), false, false},
					{solana.MustPublicKeyFromBase58(toATA.String()), false, true},
					{solana.MustPublicKeyFromBase58(fromPubkey.String()), true, false},
				},
				data: binaryData,
			},
		},
		dummyBlockhash,
		solana.TransactionPayer(solana.MustPublicKeyFromBase58(fromPubkey.String())),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	signedTx, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return &TransactionData{
		Serialized:         base64.StdEncoding.EncodeToString(signedTx),
		Instructions:       instructions,
		RecentBlockhash:    "11111111111111111111111111111111",
		FeePayer:           fromPubkey.String(),
		RequiredSignatures: []string{fromPubkey.String()},
	}, nil
}

func BuildMerkleTreeTransaction(ownerWallet string, depth uint8, maxBufferSize uint8) (*TransactionData, error) {
	ownerPubkey, err := solana.PublicKeyFromBase58(ownerWallet)
	if err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}

	treePubkey := solana.NewWallet().PublicKey()

	instruction := InstructionData{
		ProgramID:       BubblegumProgram,
		InstructionType: "init_empty_merkle_tree",
		Accounts: []AccountMeta{
			{Pubkey: treePubkey.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: ownerPubkey.String(), IsSigner: true, IsWritable: false, IsPayer: true},
		},
		Data: fmt.Sprintf("%x%x%x", depth, maxBufferSize, ownerPubkey.Bytes()),
	}

	return &TransactionData{
		Instructions:       []InstructionData{instruction},
		FeePayer:           ownerPubkey.String(),
		RequiredSignatures: []string{ownerPubkey.String()},
	}, nil
}

func BuildMintNFTTransaction(ownerWallet, name, price, description, sendTo, treeId string) (*TransactionData, error) {
	ownerPubkey, err := solana.PublicKeyFromBase58(ownerWallet)
	if err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}

	sendToPubkey, err := solana.PublicKeyFromBase58(sendTo)
	if err != nil {
		return nil, fmt.Errorf("invalid send_to address: %w", err)
	}

	treePubkey, err := solana.PublicKeyFromBase58(treeId)
	if err != nil {
		return nil, fmt.Errorf("invalid tree ID: %w", err)
	}

	bubblegumProgram, err := solana.PublicKeyFromBase58(BubblegumProgram)
	if err != nil {
		return nil, fmt.Errorf("invalid bubblegum program: %w", err)
	}

	treeAuthority := DeriveTreeAuthority(treePubkey, bubblegumProgram)

	metadata := map[string]interface{}{
		"name":                    name,
		"symbol":                  "CHORE",
		"uri":                     fmt.Sprintf("data:application/json;base64,%s", base64.StdEncoding.EncodeToString([]byte(description))),
		"seller_fee_basis_points": 0,
		"is_mutable":              true,
	}

	instruction := InstructionData{
		ProgramID:       BubblegumProgram,
		InstructionType: "mint_v1",
		Accounts: []AccountMeta{
			{Pubkey: treePubkey.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: treeAuthority.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: ownerPubkey.String(), IsSigner: true, IsWritable: false, IsPayer: true},
			{Pubkey: sendToPubkey.String(), IsSigner: false, IsWritable: false, IsPayer: false},
		},
		Data: fmt.Sprintf("%v", metadata),
	}

	return &TransactionData{
		Instructions:       []InstructionData{instruction},
		FeePayer:           ownerPubkey.String(),
		RequiredSignatures: []string{ownerPubkey.String()},
	}, nil
}

func BuildUpdateNFTTransaction(nftAddress, newStatus, sendTo string) (*TransactionData, error) {
	nftPubkey, err := solana.PublicKeyFromBase58(nftAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid NFT address: %w", err)
	}

	sendToPubkey, err := solana.PublicKeyFromBase58(sendTo)
	if err != nil {
		return nil, fmt.Errorf("invalid send_to address: %w", err)
	}

	instruction := InstructionData{
		ProgramID:       BubblegumProgram,
		InstructionType: "update_metadata",
		Accounts: []AccountMeta{
			{Pubkey: nftPubkey.String(), IsSigner: false, IsWritable: true, IsPayer: false},
			{Pubkey: sendToPubkey.String(), IsSigner: false, IsWritable: false, IsPayer: false},
		},
		Data: fmt.Sprintf("status:%s", newStatus),
	}

	return &TransactionData{
		Instructions:       []InstructionData{instruction},
		FeePayer:           nftPubkey.String(),
		RequiredSignatures: []string{nftPubkey.String()},
	}, nil
}

func BuildAcceptNFTTransaction(nftAddress, senderWallet string, paymentAmount uint64) (*TransactionData, error) {
	nftPubkey, err := solana.PublicKeyFromBase58(nftAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid NFT address: %w", err)
	}

	senderPubkey, err := solana.PublicKeyFromBase58(senderWallet)
	if err != nil {
		return nil, fmt.Errorf("invalid sender address: %w", err)
	}

	burnInstruction := InstructionData{
		ProgramID:       BubblegumProgram,
		InstructionType: "burn",
		Accounts: []AccountMeta{
			{Pubkey: nftPubkey.String(), IsSigner: false, IsWritable: true, IsPayer: false},
		},
		Data: "burn",
	}

	transferInstruction := InstructionData{
		ProgramID:       TokenProgram,
		InstructionType: "transfer",
		Accounts: []AccountMeta{
			{Pubkey: senderPubkey.String(), IsSigner: false, IsWritable: true, IsPayer: false},
		},
		Data: fmt.Sprintf("%x", paymentAmount),
	}

	return &TransactionData{
		Instructions:       []InstructionData{burnInstruction, transferInstruction},
		FeePayer:           nftPubkey.String(),
		RequiredSignatures: []string{nftPubkey.String()},
	}, nil
}

func DeriveAssociatedTokenAddress(owner solana.PublicKey, mint solana.PublicKey) (solana.PublicKey, error) {
	tokenProgramID, err := solana.PublicKeyFromBase58(TokenProgram)
	if err != nil {
		return solana.PublicKey{}, err
	}

	ataProgramID, err := solana.PublicKeyFromBase58(AssociatedTokenProgram)
	if err != nil {
		return solana.PublicKey{}, err
	}

	ata, _, err := solana.FindProgramAddress(
		[][]byte{
			owner.Bytes(),
			tokenProgramID.Bytes(),
			mint.Bytes(),
		},
		ataProgramID,
	)
	return ata, err
}

func DeriveTreeAuthority(treeID solana.PublicKey, programID solana.PublicKey) solana.PublicKey {
	treeBytes := treeID.Bytes()
	programBytes := programID.Bytes()

	combined := append(treeBytes, programBytes...)

	return solana.PublicKeyFromBytes(combined)
}
