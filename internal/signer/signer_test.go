package signer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestSigner_SignOrder(t *testing.T) {
	// Generate a random key for testing
	key, _ := crypto.GenerateKey()
	keyBytes := crypto.FromECDSA(key)
	keyHex := hexutil.Encode(keyBytes)[2:] // Remove 0x

	signer, err := NewSigner(keyHex, 137)
	assert.NoError(t, err)

	order := &Order{
		Salt:          big.NewInt(123),
		Maker:         signer.Address(),
		Signer:        signer.Address(),
		Taker:         common.Address{},
		TokenID:       big.NewInt(999),
		MakerAmount:   big.NewInt(1000000),
		TakerAmount:   big.NewInt(500000),
		Expiration:    big.NewInt(1800000000),
		Nonce:         big.NewInt(1),
		FeeRateBps:    big.NewInt(0),
		Side:          0,
		SignatureType: 0,
	}

	sig, err := signer.SignOrder(order)
	assert.NoError(t, err)
	assert.NotEmpty(t, sig)
	assert.Equal(t, 132, len(sig)) // 0x + 65 bytes * 2 = 132
}

func BenchmarkSignOrder(b *testing.B) {
	// Generate a random key for testing
	key, _ := crypto.GenerateKey()
	keyBytes := crypto.FromECDSA(key)
	keyHex := hexutil.Encode(keyBytes)[2:]

	signer, _ := NewSigner(keyHex, 137)

	order := &Order{
		Salt:          big.NewInt(123),
		Maker:         signer.Address(),
		Signer:        signer.Address(),
		Taker:         common.Address{},
		TokenID:       big.NewInt(999),
		MakerAmount:   big.NewInt(1000000),
		TakerAmount:   big.NewInt(500000),
		Expiration:    big.NewInt(1800000000),
		Nonce:         big.NewInt(1),
		FeeRateBps:    big.NewInt(0),
		Side:          0,
		SignatureType: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signer.SignOrder(order)
	}
}
