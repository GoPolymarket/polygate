package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
)

const eip1271MagicValue = "0x1626ba7e"

type EIP1271Verifier struct {
	rpcURL   string
	mu       sync.Mutex
	client   *ethclient.Client
	cacheTTL time.Duration
	cache    map[string]cacheEntry
	timeout  time.Duration
	retries  int
}

type cacheEntry struct {
	valid   bool
	expires time.Time
}

func NewEIP1271Verifier(rpcURL string, ttl time.Duration, timeout time.Duration, retries int) *EIP1271Verifier {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if retries < 0 {
		retries = 0
	}
	return &EIP1271Verifier{
		rpcURL:   strings.TrimSpace(rpcURL),
		cacheTTL: ttl,
		cache:    make(map[string]cacheEntry),
		timeout:  timeout,
		retries:  retries,
	}
}

func (v *EIP1271Verifier) Verify(ctx context.Context, contractAddr string, hash []byte, signature string) (bool, error) {
	if v.rpcURL == "" {
		return false, fmt.Errorf("rpc url not configured")
	}
	if !common.IsHexAddress(contractAddr) {
		return false, fmt.Errorf("invalid contract address")
	}
	if len(hash) != 32 {
		return false, fmt.Errorf("invalid hash length")
	}
	sigBytes, err := hexutil.Decode(signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding")
	}
	cacheKey := v.cacheKey(contractAddr, hash, signature)
	if hit, ok := v.cacheGet(cacheKey); ok {
		return hit, nil
	}

	parsedABI, err := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[{"name":"_hash","type":"bytes32"},{"name":"_signature","type":"bytes"}],"name":"isValidSignature","outputs":[{"name":"magicValue","type":"bytes4"}],"payable":false,"stateMutability":"view","type":"function"}]`))
	if err != nil {
		return false, fmt.Errorf("failed to parse abi")
	}
	contract := common.HexToAddress(contractAddr)
	data, err := parsedABI.Pack("isValidSignature", [32]byte(hash), sigBytes)
	if err != nil {
		return false, fmt.Errorf("failed to pack call data")
	}

	var lastErr error
	for attempt := 0; attempt <= v.retries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, v.timeout)
		client, err := v.getClient(attemptCtx)
		if err != nil {
			cancel()
			lastErr = err
			if !shouldRetry(ctx, attempt, v.retries) {
				break
			}
			continue
		}

		msg := ethereum.CallMsg{
			To:   &contract,
			Data: data,
		}
		output, err := client.CallContract(attemptCtx, msg, nil)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("rpc call failed: %w", err)
			if !shouldRetry(ctx, attempt, v.retries) {
				break
			}
			continue
		}
		if len(output) < 4 {
			v.cacheSet(cacheKey, false)
			return false, nil
		}
		valid := strings.EqualFold(hexutil.Encode(output[:4]), eip1271MagicValue)
		v.cacheSet(cacheKey, valid)
		return valid, nil
	}
	return false, lastErr
}

func (v *EIP1271Verifier) getClient(ctx context.Context) (*ethclient.Client, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.client != nil {
		return v.client, nil
	}
	client, err := ethclient.DialContext(ctx, v.rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect rpc: %w", err)
	}
	v.client = client
	return v.client, nil
}

func (v *EIP1271Verifier) cacheKey(contractAddr string, hash []byte, signature string) string {
	return strings.ToLower(contractAddr) + ":" + hexutil.Encode(hash) + ":" + strings.ToLower(signature)
}

func (v *EIP1271Verifier) cacheGet(key string) (bool, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.cache[key]
	if !ok {
		return false, false
	}
	if time.Now().After(entry.expires) {
		delete(v.cache, key)
		return false, false
	}
	return entry.valid, true
}

func (v *EIP1271Verifier) cacheSet(key string, valid bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache[key] = cacheEntry{
		valid:   valid,
		expires: time.Now().Add(v.cacheTTL),
	}
}

func shouldRetry(ctx context.Context, attempt, max int) bool {
	if attempt >= max {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	default:
	}
	time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	return true
}
