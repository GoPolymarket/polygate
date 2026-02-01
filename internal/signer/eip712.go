package signer

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Constants for EIP-712
const (
	EIP712DomainName    = "Polymarket CTF Exchange"
	EIP712DomainVersion = "1"
	
	// Exchange Contract Address on Polygon
	ExchangeContractAddress = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
)

var (
	// EIP712DomainTypeHash is the keccak256 hash of the EIP712Domain type definition
	// "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
	EIP712DomainTypeHash = crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))

	// OrderTypeHash is the keccak256 hash of the Order type definition
	// "clobtypes.Order(uint256 salt,address maker,address signer,address taker,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint256 expiration,uint256 nonce,uint256 feeRateBps,uint8 side,uint8 signatureType)"
	OrderTypeHash = crypto.Keccak256Hash([]byte("clobtypes.Order(uint256 salt,address maker,address signer,address taker,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint256 expiration,uint256 nonce,uint256 feeRateBps,uint8 side,uint8 signatureType)"))
)

// Order represents the struct to be signed
type Order struct {
	Salt          *big.Int
	Maker         common.Address
	Signer        common.Address
	Taker         common.Address
	TokenID       *big.Int
	MakerAmount   *big.Int
	TakerAmount   *big.Int
	Expiration    *big.Int
	Nonce         *big.Int
	FeeRateBps    *big.Int
	Side          uint8
	SignatureType uint8
}
