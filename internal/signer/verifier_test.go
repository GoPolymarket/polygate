package signer

import (
	"math/big"
	"testing"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"
	sdktypes "github.com/GoPolymarket/polymarket-go-sdk/pkg/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestVerifyOrderSignature(t *testing.T) {
	// 1. Setup Signer
	key, _ := crypto.GenerateKey()
	keyBytes := crypto.FromECDSA(key)
	keyHex := hexutil.Encode(keyBytes)[2:]
	
	s, err := NewSigner(keyHex, 137)
	assert.NoError(t, err)
	signerAddr := s.Address()

	// 2. Setup Order (SDK type)
	order := &clobtypes.Order{
		Salt:        sdktypes.U256{Int: big.NewInt(123)},
		Maker:       signerAddr,
		Signer:      signerAddr,
		Taker:       common.Address{},
		TokenID:     sdktypes.U256{Int: big.NewInt(999)},
		MakerAmount: decimal.NewFromInt(1000000),
		TakerAmount: decimal.NewFromInt(500000),
		Expiration:  sdktypes.U256{Int: big.NewInt(1800000000)},
		Nonce:       sdktypes.U256{Int: big.NewInt(1)},
		FeeRateBps:  decimal.NewFromInt(0),
		Side:        "BUY",
	}

	// 3. Sign using Optimized Signer
	optOrder := &Order{
		Salt:          order.Salt.Int,
		Maker:         order.Maker,
		Signer:        order.Signer,
		Taker:         order.Taker,
		TokenID:       order.TokenID.Int,
		MakerAmount:   order.MakerAmount.BigInt(),
		TakerAmount:   order.TakerAmount.BigInt(),
		Expiration:    order.Expiration.Int,
		Nonce:         order.Nonce.Int,
		FeeRateBps:    order.FeeRateBps.BigInt(),
		Side:          0, // BUY
		SignatureType: 0, // EOA
	}
	
	sig, err := s.SignOrder(optOrder)
	assert.NoError(t, err)

	// 4. Verify using Verifier
	err = VerifyOrderSignature(order, sig, signerAddr.Hex(), 137)
	assert.NoError(t, err)

	// 5. Test failure with wrong signer
	wrongAddr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	err = VerifyOrderSignature(order, sig, wrongAddr.Hex(), 137)
	assert.Error(t, err)
}
