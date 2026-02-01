package service

import (
	"context"
	"fmt"

	relayer "github.com/GoPolymarket/go-builder-relayer-client"
	"github.com/GoPolymarket/go-builder-relayer-client/pkg/signer"
	"github.com/GoPolymarket/go-builder-relayer-client/pkg/types"
	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/ethereum/go-ethereum/common"
)

// AccountService 处理账户相关逻辑，如 Proxy 部署
type AccountService struct {
	tm             *TenantManager
	relayClient    *relayer.RelayClient
	builderConfig  *relayer.BuilderConfig
	relayerBaseURL string
	relayerChainID int64
}

func NewAccountService(tm *TenantManager, relayClient *relayer.RelayClient, builderConfig *relayer.BuilderConfig, relayerConfig config.RelayerConfig) *AccountService {
	return &AccountService{
		tm:             tm,
		relayClient:    relayClient,
		builderConfig:  builderConfig,
		relayerBaseURL: relayerConfig.BaseURL,
		relayerChainID: relayerConfig.ChainID,
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
		signer, _ := auth.NewPrivateKeySigner(tenant.Creds.PrivateKey, s.relayerChainID)
		signerAddr = signer.Address()
	}

	proxyAddr, err := auth.DeriveProxyWalletForChain(signerAddr, s.relayerChainID)
	if err != nil {
		return nil, err
	}

	return &ProxyStatusResponse{
		IsReady:      true, // Assume ready if derived for demo
		ProxyAddress: proxyAddr.Hex(),
	}, nil
}

type DeployProxyResult struct {
	TransactionID string `json:"transaction_id"`
	SafeAddress   string `json:"safe_address,omitempty"`
}

// DeployProxy 通过 Relayer 部署 Safe (Gasless)
func (s *AccountService) DeployProxy(ctx context.Context, tenant *model.Tenant) (*DeployProxyResult, error) {
	if tenant.Creds.PrivateKey == "" {
		return nil, fmt.Errorf("private key required for signing")
	}

	// Create signer from tenant's private key
	pkSigner, err := signer.NewPrivateKeySigner(tenant.Creds.PrivateKey, s.relayerChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	logger.Info("Deploying Safe for tenant via Relayer", "tenant_id", tenant.ID)

	// Create relay client with the signer
	relayClient, err := relayer.NewRelayClient(
		s.relayerBaseURL,
		s.relayerChainID,
		pkSigner,
		s.builderConfig,
		types.RelayerTxSafe,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create relay client: %w", err)
	}

	// Build transaction to deploy Safe
	// For Safe deployment, we need to call the Safe factory
	tx := types.Transaction{
		To:    "0x0000000000000000000000000000000000000000", // Placeholder - actual factory address
		Data:  "0x",                                         // Placeholder - actual factory call data
		Value: "0",
	}

	// Execute transaction via relayer
	resp, err := relayClient.Execute(ctx, []types.Transaction{tx}, "Deploying Polymarket Safe")
	if err != nil {
		return nil, fmt.Errorf("relayer execution failed: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("relayer returned nil response")
	}

	var safeAddress string
	if addr, err := auth.DeriveSafeWalletForChain(pkSigner.Address(), s.relayerChainID); err == nil {
		safeAddress = addr.Hex()
	} else {
		logger.Warn("Failed to derive safe address", "error", err)
	}

	return &DeployProxyResult{
		TransactionID: resp.TransactionID,
		SafeAddress:   safeAddress,
	}, nil
}
