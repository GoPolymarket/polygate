package signer

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
)

type Signer struct {
	key             *ecdsa.PrivateKey
	address         common.Address
	chainID         *big.Int
	domainSeparator common.Hash
}

// NewSigner creates a new EIP-712 signer with pre-calculated domain separator
func NewSigner(privateKeyHex string, chainID int64) (*Signer, error) {
	// 1. Parse Private Key
	if privateKeyHex == "" {
		return nil, fmt.Errorf("private key is required")
	}
	key, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}

	// 2. Derive Address
	publicKey := key.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 3. Pre-calculate Domain Separator
	// keccak256(abi.encode(EIP712DomainTypeHash, keccak256("Polymarket CTF Exchange"), keccak256("1"), chainId, verifyingContract))
	domainNameHash := crypto.Keccak256Hash([]byte(EIP712DomainName))
	versionHash := crypto.Keccak256Hash([]byte(EIP712DomainVersion))
	
	// Manual ABI Encode for Domain Separator to avoid reflection overhead
	// All fields are 32 bytes
	domainData := make([]byte, 32*5)
	copy(domainData[0:32], EIP712DomainTypeHash.Bytes())
	copy(domainData[32:64], domainNameHash.Bytes())
	copy(domainData[64:96], versionHash.Bytes())
	
	// ChainID (uint256)
	bChainID := math.U256Bytes(big.NewInt(chainID))
	// Pad to 32 bytes (math.U256Bytes already does 32 bytes)
	copy(domainData[96:128], bChainID)

	// Verifying Contract (address -> uint256 padded)
	// common.HexToAddress returns 20 bytes, need to pad to left
	verifyingAddr := common.HexToAddress(ExchangeContractAddress)
	copy(domainData[128+12:160], verifyingAddr.Bytes()) // last 20 bytes

	domainSeparator := crypto.Keccak256Hash(domainData)

	return &Signer{
		key:             key,
		address:         address,
		chainID:         big.NewInt(chainID),
		domainSeparator: domainSeparator,
	}, nil
}

// SignOrder calculates the EIP-712 hash and signs it
// Returns (r, s, v) as per standard ECDSA signature
func (s *Signer) SignOrder(order *Order) (string, error) {
	// 1. Calculate HashStruct(Order)
	hashStruct, err := s.hashOrder(order)
	if err != nil {
		return "", err
	}

	// 2. Calculate EIP-191 Hash: keccak256("\x19\x01" + domainSeparator + hashStruct)
	finalHash := crypto.Keccak256([]byte{0x19, 0x01}, s.domainSeparator.Bytes(), hashStruct)

	// 3. Sign
	signature, err := crypto.Sign(finalHash, s.key)
	if err != nil {
		return "", err
	}

	// 4. Adjust V (Go Ethereum produces 0/1, we typically need 27/28 for some verifiers, 
	// but Polymarket/EIP-712 usually accepts standard recovery id. 
	// Note: crypto.Sign returns [R || S || V] where V is 0 or 1.
	// Standard Ethereum RPC often expects V to be 27 or 28 (old style) or just 0/1 depending on replay protection.
	// EIP-712 usually expects standard 65 byte sig.
	if signature[64] < 27 {
		signature[64] += 27
	}

	// Return Hex string
	return "0x" + common.Bytes2Hex(signature), nil
}

// hashOrder calculates hashStruct(order)
// keccak256(abi.encode(typeHash, salt, maker, ...))
func (s *Signer) hashOrder(order *Order) ([]byte, error) {
	// Order has 12 fields + typeHash = 13 items * 32 bytes = 416 bytes
	data := make([]byte, 32*13)

	// 0. TypeHash
	copy(data[0:32], OrderTypeHash.Bytes())

	// 1. Salt (uint256)
	if order.Salt != nil {
		copy(data[32:64], math.U256Bytes(order.Salt))
	}

	// 2. Maker (address)
	copy(data[64+12:96], order.Maker.Bytes())

	// 3. Signer (address)
	copy(data[96+12:128], order.Signer.Bytes())

	// 4. Taker (address)
	copy(data[128+12:160], order.Taker.Bytes())

	// 5. TokenID (uint256)
	if order.TokenID != nil {
		copy(data[160:192], math.U256Bytes(order.TokenID))
	}

	// 6. MakerAmount (uint256)
	if order.MakerAmount != nil {
		copy(data[192:224], math.U256Bytes(order.MakerAmount))
	}

	// 7. TakerAmount (uint256)
	if order.TakerAmount != nil {
		copy(data[224:256], math.U256Bytes(order.TakerAmount))
	}

	// 8. Expiration (uint256)
	if order.Expiration != nil {
		copy(data[256:288], math.U256Bytes(order.Expiration))
	}

	// 9. Nonce (uint256)
	if order.Nonce != nil {
		copy(data[288:320], math.U256Bytes(order.Nonce))
	}

	// 10. FeeRateBps (uint256)
	if order.FeeRateBps != nil {
		copy(data[320:352], math.U256Bytes(order.FeeRateBps))
	}

	// 11. Side (uint8 -> uint256)
	// Side is 0 (Buy) or 1 (Sell) in typical Polymarket logic, but here we just encode what is given
	copy(data[352:384], math.U256Bytes(big.NewInt(int64(order.Side))))

	// 12. SignatureType (uint8 -> uint256)
	copy(data[384:416], math.U256Bytes(big.NewInt(int64(order.SignatureType))))

	return crypto.Keccak256(data), nil
}

func (s *Signer) Address() common.Address {
	return s.address
}
