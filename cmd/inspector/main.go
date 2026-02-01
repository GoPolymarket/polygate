package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"os"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/inspector <private_key_with_0x>")
		os.Exit(1)
	}

	pkHex := os.Args[1]
	
	// 1. Validate Private Key
	key, err := crypto.HexToECDSA(pkHex[2:]) // remove 0x
	if err != nil {
		log.Fatalf("❌ Invalid Private Key: %v", err)
	}
	
	// 2. Derive EOA Address
	pubKey := key.Public()
	eoaAddr := crypto.PubkeyToAddress(*pubKey.(*ecdsa.PublicKey))
	fmt.Printf("\n✅ Private Key is Valid!\n")
	fmt.Printf("🔑 EOA Address (MetaMask):   %s\n", eoaAddr.Hex())

	// 3. Derive Proxy Address (The one Polymarket uses)
	// Polymarket uses a specific Proxy Factory. The SDK knows how to derive it.
	// Try standard proxy derivation
	proxyAddr, err := auth.DeriveProxyWallet(eoaAddr)
	if err != nil {
		fmt.Printf("⚠️  Could not derive proxy (Calculation Error): %v\n", err)
	} else {
		fmt.Printf("🏭 Proxy Address (Polymarket): %s\n", proxyAddr.Hex())
		fmt.Println("\n👇 COPY THIS TO config.yaml 👇")
		fmt.Printf("proxy_address: \"%s\"\n", proxyAddr.Hex())
	}
}