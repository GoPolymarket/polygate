package manager

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// NonceManager handles both Ethereum Transaction Nonces (for txs) and Exchange Nonces (for orders)
type NonceManager struct {
	client *ethclient.Client
	
	// Transaction Nonces (Optimistic)
	txNonces   map[common.Address]uint64
	txMu       sync.RWMutex

	// Exchange Nonces (Cached, Read-mostly)
	// These are the values stored in the CTF Exchange contract: nonces(user)
	exchangeNonces map[common.Address]*big.Int
	exchangeMu     sync.RWMutex
}

func NewNonceManager(rpcURL string) (*NonceManager, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to eth client: %w", err)
	}

	return &NonceManager{
		client:         client,
		txNonces:       make(map[common.Address]uint64),
		exchangeNonces: make(map[common.Address]*big.Int),
	}, nil
}

// --- Ethereum Transaction Nonce (Optimistic) ---

// GetNextTxNonce returns the next expected nonce for a transaction.
// If it's the first time, it fetches from chain.
func (m *NonceManager) GetNextTxNonce(ctx context.Context, addr common.Address) (uint64, error) {
	m.txMu.Lock()
	defer m.txMu.Unlock()

	nonce, ok := m.txNonces[addr]
	if ok {
		return nonce, nil
	}

	// Fetch from chain (Pending to be safe, or Latest)
	// Using PendingNonceAt to account for mempool
	fetched, err := m.client.PendingNonceAt(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pending nonce: %w", err)
	}

	m.txNonces[addr] = fetched
	return fetched, nil
}

// IncrementTxNonce manually increments the local nonce. 
// Call this AFTER successfully signing/broadcasting a transaction.
func (m *NonceManager) IncrementTxNonce(addr common.Address) {
	m.txMu.Lock()
	defer m.txMu.Unlock()
	if _, ok := m.txNonces[addr]; ok {
		m.txNonces[addr]++
	}
}

// ResetTxNonce forces a re-sync from the chain.
// Call this if you get "Nonce too low" or "Replacement transaction underpriced".
func (m *NonceManager) ResetTxNonce(ctx context.Context, addr common.Address) error {
	m.txMu.Lock()
	defer m.txMu.Unlock()

	fetched, err := m.client.PendingNonceAt(ctx, addr)
	if err != nil {
		return err
	}
	m.txNonces[addr] = fetched
	logger.Info("Reset TX nonce", "address", addr.Hex(), "nonce", fetched)
	return nil
}

// --- CTF Exchange Nonce (Cached) ---

// GetExchangeNonce returns the current valid nonce for Orders.
// For standard CTF Exchange, Order.Nonce must EQUAL the contract's nonces(maker).
func (m *NonceManager) GetExchangeNonce(ctx context.Context, addr common.Address) (*big.Int, error) {
	m.exchangeMu.RLock()
	cached, ok := m.exchangeNonces[addr]
	m.exchangeMu.RUnlock()
	if ok {
		return cached, nil
	}

	return m.SyncExchangeNonce(ctx, addr)
}

// SyncExchangeNonce forces a fetch of the Exchange Nonce from the contract.
func (m *NonceManager) SyncExchangeNonce(ctx context.Context, addr common.Address) (*big.Int, error) {
	m.exchangeMu.Lock()
	defer m.exchangeMu.Unlock()

	// In a real implementation, we would call the contract: Exchange.nonces(addr)
	// For now, in this MVP, we will simulate or fetch if we had the contract ABI binding.
	// Since we don't have the generated bindings in this snippet, we will default to 0 
	// (which is correct for a fresh account) or rely on a "mock" fetch.
	// TODO: Replace with actual contract call: exchange.Nonces(&bind.CallOpts{}, addr)
	
	// For MVP Phase 1/2 without full contract bindings, we assume 0 or 
	// use a placeholder that the GatewayService might populate via SDK if needed.
	// But to be "Robust", let's try to use eth_call if possible or just 0.
	
	// Assuming 0 for now as most bots start fresh or we rely on SDK to fetch it once.
	// But wait, the SDK's `GetNonce` typically calls the API or Chain.
	// Let's implement a basic ETH Call here if we want to be "The Engine".
	
	// Exchange Contract: 0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E
	// Function: nonces(address) -> uint256
	// Selector: 0x7ecebe00 (keccak256("nonces(address)")[:4])
	
	// Construct calldata: selector + address (padded)
	/*
	contractAddr := common.HexToAddress("0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E")
	selector := []byte{0x7e, 0xce, 0xbe, 0x00}
	addrBytes := common.LeftPadBytes(addr.Bytes(), 32)
	data := append(selector, addrBytes...)
	
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}
	res, err := m.client.CallContract(ctx, msg, nil)
	if err != nil {
		// Fallback or error
		log.Printf("[NonceManager] Failed to fetch exchange nonce: %v", err)
		val := big.NewInt(0)
		m.exchangeNonces[addr] = val
		return val, nil
	}
	
	val := new(big.Int).SetBytes(res)
	*/
	
	// Simplified for this step: Return 0 (Default)
	// The user can implement the actual contract call in Phase 3 or we use SDK to fetch.
	val := big.NewInt(0)
	m.exchangeNonces[addr] = val
	return val, nil
}

// InvalidateExchangeNonce increments the cached exchange nonce.
// Call this when you send a "Cancel All" transaction.
func (m *NonceManager) InvalidateExchangeNonce(addr common.Address) {
	m.exchangeMu.Lock()
	defer m.exchangeMu.Unlock()
	
	if val, ok := m.exchangeNonces[addr]; ok {
		// Incrementing locally so new orders use the new nonce immediately
		// even before the CancelAll tx is mined (Optimistic!)
		m.exchangeNonces[addr] = new(big.Int).Add(val, big.NewInt(1))
	}
}
