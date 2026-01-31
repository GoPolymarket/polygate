package service

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const exchangeContractAddress = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"

type staticSigner struct {
	address common.Address
	chainID *big.Int
}

func (s *staticSigner) Address() common.Address {
	return s.address
}

func (s *staticSigner) ChainID() *big.Int {
	return s.chainID
}

func (s *staticSigner) SignTypedData(_ *apitypes.TypedDataDomain, _ apitypes.Types, _ apitypes.TypedDataMessage, _ string) ([]byte, error) {
	return nil, errors.New("static signer cannot sign")
}

func newStaticSigner(address string, chainID int64) (*staticSigner, error) {
	if address == "" {
		return nil, fmt.Errorf("signer address is required")
	}
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("invalid signer address")
	}
	return &staticSigner{
		address: common.HexToAddress(address),
		chainID: big.NewInt(chainID),
	}, nil
}

func buildTypedData(order *clobtypes.Order, signer common.Address, chainID int64) (apitypes.TypedData, error) {
	if order == nil {
		return apitypes.TypedData{}, fmt.Errorf("order is required")
	}
	domain := apitypes.TypedDataDomain{
		Name:              "Polymarket CTF Exchange",
		Version:           "1",
		ChainId:           (*math.HexOrDecimal256)(big.NewInt(chainID)),
		VerifyingContract: exchangeContractAddress,
	}
	typesDef := apitypes.Types{
		"EIP712Domain": {
			{Name: "name", Type: "string"},
			{Name: "version", Type: "string"},
			{Name: "chainId", Type: "uint256"},
			{Name: "verifyingContract", Type: "address"},
		},
		"clobtypes.Order": {
			{Name: "salt", Type: "uint256"},
			{Name: "maker", Type: "address"},
			{Name: "signer", Type: "address"},
			{Name: "taker", Type: "address"},
			{Name: "tokenId", Type: "uint256"},
			{Name: "makerAmount", Type: "uint256"},
			{Name: "takerAmount", Type: "uint256"},
			{Name: "expiration", Type: "uint256"},
			{Name: "nonce", Type: "uint256"},
			{Name: "feeRateBps", Type: "uint256"},
			{Name: "side", Type: "uint8"},
			{Name: "signatureType", Type: "uint8"},
		},
	}

	sideInt := 0
	if strings.ToUpper(order.Side) == "SELL" {
		sideInt = 1
	}
	sigType := 0
	if order.SignatureType != nil {
		sigType = *order.SignatureType
	}
	expiration := big.NewInt(0)
	if order.Expiration.Int != nil {
		expiration = order.Expiration.Int
	}

	message := apitypes.TypedDataMessage{
		"salt":          (*math.HexOrDecimal256)(order.Salt.Int),
		"maker":         order.Maker.String(),
		"signer":        signer.String(),
		"taker":         order.Taker.String(),
		"tokenId":       (*math.HexOrDecimal256)(order.TokenID.Int),
		"makerAmount":   (*math.HexOrDecimal256)(order.MakerAmount.BigInt()),
		"takerAmount":   (*math.HexOrDecimal256)(order.TakerAmount.BigInt()),
		"expiration":    (*math.HexOrDecimal256)(expiration),
		"nonce":         (*math.HexOrDecimal256)(order.Nonce.Int),
		"feeRateBps":    (*math.HexOrDecimal256)(order.FeeRateBps.BigInt()),
		"side":          (*math.HexOrDecimal256)(big.NewInt(int64(sideInt))),
		"signatureType": (*math.HexOrDecimal256)(big.NewInt(int64(sigType))),
	}

	return apitypes.TypedData{
		Types:       typesDef,
		PrimaryType: "clobtypes.Order",
		Domain:      domain,
		Message:     message,
	}, nil
}

func verifyOrderSignature(order *clobtypes.Order, signature string, signerAddr string, chainID int64) error {
	if order == nil {
		return fmt.Errorf("order is required")
	}
	if signature == "" {
		return fmt.Errorf("signature is required")
	}
	if !common.IsHexAddress(signerAddr) {
		return fmt.Errorf("invalid signer address")
	}
	signer := common.HexToAddress(signerAddr)
	hash, err := typedDataHash(order, signer, chainID)
	if err != nil {
		return fmt.Errorf("failed to hash typed data: %w", err)
	}
	rawSig, err := hexutil.Decode(signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	if len(rawSig) != 65 {
		return fmt.Errorf("invalid signature length")
	}
	// Normalize V to 0/1 for recovery.
	if rawSig[64] >= 27 {
		rawSig[64] -= 27
	}
	pub, err := crypto.SigToPub(hash, rawSig)
	if err != nil {
		return fmt.Errorf("signature recovery failed")
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if recovered != signer {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func typedDataHash(order *clobtypes.Order, signer common.Address, chainID int64) ([]byte, error) {
	typedData, err := buildTypedData(order, signer, chainID)
	if err != nil {
		return nil, err
	}
	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func signatureTypeSupported(sigType *int) bool {
	if sigType == nil {
		return true
	}
	switch auth.SignatureType(*sigType) {
	case auth.SignatureEOA, auth.SignatureProxy:
		return true
	default:
		return false
	}
}
