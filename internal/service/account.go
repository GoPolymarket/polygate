package service

import (
	"context"
	"fmt"
	"log"

	"github.com/GoPolymarket/go-builder-relayer-client/pkg/signer"
	"github.com/GoPolymarket/go-builder-relayer-client/pkg/types"
	relayer "github.com/GoPolymarket/go-builder-relayer-client"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/ethereum/go-ethereum/common"
)

// AccountService 处理账户相关逻辑，如 Proxy 部署
type AccountService struct {
	tm            *TenantManager
	relayClient   *relayer.RelayClient
	builderConfig *relayer.BuilderConfig
}

func NewAccountService(tm *TenantManager, relayClient *relayer.RelayClient, builderConfig *relayer.BuilderConfig) *AccountService {
	return &AccountService{
		tm:            tm,
		relayClient:   relayClient,
		builderConfig: builderConfig,
	}
}

type ProxyStatusResponse struct {
	IsReady      bool   `json:"is_ready"`
	ProxyAddress string `json:"proxy_address,omitempty"`
}

// GetProxyStatus 检查租户是否已部署 Proxy Wallet
func (s *AccountService) GetProxyStatus(ctx context.Context, tenant *model.Tenant) (*ProxyStatusResponse, error) {
	// For MVP, we derive the proxy address using the SDK's auth package.
	// In production, you would check on-chain for deployment.
	signerAddr := common.HexToAddress(tenant.Creds.Address)
	if tenant.Creds.Address == "" && tenant.Creds.PrivateKey != "" {
		signer, _ := auth.NewPrivateKeySigner(tenant.Creds.PrivateKey, 137)
		signerAddr = signer.Address()
	}

	proxyAddr, err := auth.DeriveProxyWallet(signerAddr)
	if err != nil {
		return nil, err
	}

	return &ProxyStatusResponse{
		IsReady:      true, // Assume ready if derived for demo
		ProxyAddress: proxyAddr.Hex(),
	}, nil
}

// DeployProxy 通过 Relayer 部署 Safe (Gasless)
func (s *AccountService) DeployProxy(ctx context.Context, tenant *model.Tenant) (string, error) {
	if tenant.Creds.PrivateKey == "" {
		return "", fmt.Errorf("private key required for signing")
	}

	// Create signer from tenant's private key
	pkSigner, err := signer.NewPrivateKeySigner(tenant.Creds.PrivateKey, 137)
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %w", err)
	}

	log.Printf("🚀 Deploying Safe for tenant %s via Relayer...", tenant.ID)

	// Create relay client with the signer
	relayClient, err := relayer.NewRelayClient(
		"https://relayer-v2.polymarket.com",
		137,
		pkSigner,
		s.builderConfig,
		types.RelayerTxSafe,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create relay client: %w", err)
	}

	// Build transaction to deploy Safe
	// For Safe deployment, we need to call the Safe factory
	tx := types.Transaction{
		To:    "0x0000000000000000000000000000000000000000", // Placeholder - actual factory address
		Data:  "0x",                                          // Placeholder - actual factory call data
		Value: "0",
	}

	// Execute transaction via relayer
	resp, err := relayClient.Execute(ctx, []types.Transaction{tx}, "Deploying Polymarket Safe")
	if err != nil {
		return "", fmt.Errorf("relayer execution failed: %w", err)
	}

	if resp == nil {
		return "", fmt.Errorf("relayer returned nil response")
	}

	return resp.TransactionID, nil
}
