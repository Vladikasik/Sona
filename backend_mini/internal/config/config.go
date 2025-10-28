package config

import (
	"fmt"
	"os"

	"github.com/gagliardetto/solana-go"
)

var ServerWallet *solana.PrivateKey

func LoadServerWallet() error {
	privKeyStr := os.Getenv("SERVER_WALLET_PRIVATE_KEY")
	if privKeyStr == "" {
		return fmt.Errorf("SERVER_WALLET_PRIVATE_KEY environment variable not set")
	}

	privKey, err := solana.PrivateKeyFromBase58(privKeyStr)
	if err != nil {
		return fmt.Errorf("failed to parse server wallet private key: %w", err)
	}

	ServerWallet = &privKey
	return nil
}

func GetServerWallet() (*solana.PrivateKey, error) {
	if ServerWallet == nil {
		return nil, fmt.Errorf("server wallet not loaded")
	}
	return ServerWallet, nil
}
