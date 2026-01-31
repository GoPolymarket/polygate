package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/model"

	"github.com/GoPolymarket/polymarket-go-sdk"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"
	"github.com/shopspring/decimal"
)

type GatewayService struct {
	tm      *TenantManager
	risk    *RiskEngine
	config  *config.Config
	rpcURL  string
	eip1271 *EIP1271Verifier
}

func NewGatewayService(cfg *config.Config, tm *TenantManager, risk *RiskEngine) (*GatewayService, error) {
	return &GatewayService{
		tm:     tm,
		risk:   risk,
		config: cfg,
		rpcURL: cfg.Chain.RPCURL,
	}, nil
}

// OrderRequest represents the incoming JSON body
type OrderRequest struct {
	TokenID       string                   `json:"token_id" binding:"required"`
	Price         float64                  `json:"price" binding:"required"`
	Size          float64                  `json:"size" binding:"required"`
	Side          string                   `json:"side" binding:"required,oneof=BUY SELL"` // BUY or SELL
	OrderType     string                   `json:"order_type,omitempty"`                   // GTC/GTD/FAK/FOK
	PostOnly      *bool                    `json:"post_only,omitempty"`
	Expiration    int64                    `json:"expiration,omitempty"` // unix seconds (GTD)
	Signable      *clobtypes.SignableOrder `json:"signable,omitempty"`
	Signature     string                   `json:"signature,omitempty"`
	Signer        string                   `json:"signer,omitempty"`
	SignatureType *int                     `json:"signature_type,omitempty"` // 0=EOA,1=Proxy,2=Safe
	L2            *L2Creds                 `json:"l2,omitempty"`
}

type L2Creds struct {
	APIKey        string `json:"api_key"`
	APISecret     string `json:"api_secret"`
	APIPassphrase string `json:"api_passphrase"`
}

type TypedOrderResponse struct {
	Signable  *clobtypes.SignableOrder `json:"signable"`
	TypedData interface{}              `json:"typed_data"`
}

func (s *GatewayService) PlaceOrder(ctx context.Context, tenant *model.Tenant, req OrderRequest) (*clobtypes.OrderResponse, error) {
	if req.Signature != "" && req.Signable == nil {
		return nil, fmt.Errorf("signable order required when providing signature")
	}
	// 1. Resolve signable order (use provided signable for non-custodial)
	signable := req.Signable
	riskReq := req
	if signable != nil {
		if signable.Order == nil {
			return nil, fmt.Errorf("signable order is required")
		}
		riskReq = requestFromOrder(signable)
	}

	// 2. Risk Engine Check (Pre-Trade)
	if err := s.risk.CheckOrder(ctx, tenant, riskReq); err != nil {
		return nil, err
	}

	// 3. Resolve signer (custodial or non-custodial)
	var signer auth.Signer
	useGatewaySigner := false
	if strings.TrimSpace(req.Signature) == "" {
		if tenant.Creds.PrivateKey == "" {
			return nil, fmt.Errorf("signature required or tenant private key not configured")
		}
		var err error
		signer, err = auth.NewPrivateKeySigner(tenant.Creds.PrivateKey, auth.PolygonChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to create signer: %w", err)
		}
		useGatewaySigner = true
	} else {
		signerAddr := strings.TrimSpace(req.Signer)
		if signerAddr == "" && signable != nil && signable.Order != nil {
			signerAddr = signable.Order.Signer.Hex()
		}
		if signable != nil && signable.Order != nil && req.Signer != "" {
			if !strings.EqualFold(signable.Order.Signer.Hex(), req.Signer) {
				return nil, fmt.Errorf("signer does not match signable order")
			}
		}
		if signerAddr == "" {
			return nil, fmt.Errorf("signer address required when signature is provided")
		}
		if !tenantAllowsSigner(tenant, signerAddr) {
			return nil, fmt.Errorf("signer not allowed for tenant")
		}
		var err error
		signer, err = newStaticSigner(signerAddr, auth.PolygonChainID)
		if err != nil {
			return nil, err
		}
	}

	// 4. Resolve L2 credentials
	apiKey, err := resolveAPIKey(tenant, req)
	if err != nil {
		return nil, err
	}

	// 5. Build Signable Order if not provided
	client := s.newClient(nil, nil)
	if signable == nil {
		signable, err = s.buildSignable(ctx, client, signer, req)
		if err != nil {
			return nil, err
		}
	} else {
		if req.SignatureType != nil {
			sigType := *req.SignatureType
			signable.Order.SignatureType = &sigType
		}
	}

	// 6. Enforce max slippage (optional)
	if err := s.checkMaxSlippage(ctx, client, tenant, riskReq); err != nil {
		return nil, err
	}

	// 7. Execute via SDK
	execClient := s.newClient(signer, apiKey)
	var resp clobtypes.OrderResponse
	if useGatewaySigner {
		resp, err = execClient.CLOB.CreateOrderFromSignable(ctx, signable)
		if err != nil {
			return nil, fmt.Errorf("polymarket api error: %w", err)
		}
	} else {
		sigType := req.SignatureType
		if sigType == nil && signable.Order.SignatureType != nil {
			sigType = signable.Order.SignatureType
		}
		if !signatureTypeSupported(sigType) && !tenant.Risk.AllowUnverifiedSignatures {
			return nil, fmt.Errorf("signature type not supported for verification")
		}
		if sigType != nil && *sigType == int(auth.SignatureGnosisSafe) {
			if tenant.Risk.AllowUnverifiedSignatures {
				// Skip verification if explicitly allowed.
			} else {
				hash, err := typedDataHash(signable.Order, signer.Address(), auth.PolygonChainID)
				if err != nil {
					return nil, fmt.Errorf("failed to hash typed data")
				}
				verifier, err := s.getEIP1271Verifier()
				if err != nil {
					return nil, err
				}
				ok, err := verifier.Verify(ctx, signable.Order.Maker.Hex(), hash, req.Signature)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("invalid safe signature")
				}
			}
		} else if signatureTypeSupported(sigType) {
			signerAddr := strings.TrimSpace(req.Signer)
			if signerAddr == "" {
				signerAddr = signable.Order.Signer.Hex()
			}
			if err := verifyOrderSignature(signable.Order, req.Signature, signerAddr, auth.PolygonChainID); err != nil {
				return nil, fmt.Errorf("invalid signature")
			}
		}
		signed := &clobtypes.SignedOrder{
			Order:     *signable.Order,
			Signature: req.Signature,
			Owner:     apiKey.Key,
			OrderType: signable.OrderType,
			PostOnly:  signable.PostOnly,
		}
		resp, err = execClient.CLOB.PostOrder(ctx, signed)
		if err != nil {
			return nil, fmt.Errorf("polymarket api error: %w", err)
		}
	}

	// 8. Update Risk State (Post-Trade)
	s.risk.PostOrderHook(ctx, tenant, riskReq)

	return &resp, nil
}

// CancelOrderInput defines parameters for cancelling a single order
type CancelOrderInput struct {
	ID string `json:"id" binding:"required"`
}

func (s *GatewayService) CancelOrder(ctx context.Context, tenant *model.Tenant, input CancelOrderInput) (*clobtypes.CancelResponse, error) {
	// 1. Get Client
	client, err := s.tm.GetClientForTenant(tenant)
	if err != nil {
		return nil, err
	}

	// 2. Prepare Cancel Request
	req := &clobtypes.CancelOrderRequest{
		OrderID: input.ID,
	}
	
	resp, err := client.CLOB.CancelOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	return &resp, nil
}

// CancelAllOrders cancels all open orders for the tenant
func (s *GatewayService) CancelAllOrders(ctx context.Context, tenant *model.Tenant) (*clobtypes.CancelAllResponse, error) {
	client, err := s.tm.GetClientForTenant(tenant)
	if err != nil {
		return nil, err
	}

	// SDK usually takes a cancel all request object or nil
	resp, err := client.CLOB.CancelAll(ctx) // Fixed: Remove nil arg
	if err != nil {
		return nil, fmt.Errorf("failed to cancel all orders: %w", err)
	}
	
	return &resp, nil
}

func (s *GatewayService) BuildTypedOrder(ctx context.Context, tenant *model.Tenant, req OrderRequest) (*TypedOrderResponse, error) {
	if req.Signer == "" {
		return nil, fmt.Errorf("signer is required")
	}
	if !tenantAllowsSigner(tenant, req.Signer) {
		return nil, fmt.Errorf("signer not allowed for tenant")
	}
	if err := s.risk.CheckOrder(ctx, tenant, req); err != nil {
		return nil, err
	}
	signer, err := newStaticSigner(req.Signer, auth.PolygonChainID)
	if err != nil {
		return nil, err
	}
	client := s.newClient(nil, nil)
	signable, err := s.buildSignable(ctx, client, signer, req)
	if err != nil {
		return nil, err
	}
	typedData, err := buildTypedData(signable.Order, signer.Address(), auth.PolygonChainID)
	if err != nil {
		return nil, err
	}
	return &TypedOrderResponse{
		Signable:  signable,
		TypedData: typedData,
	}, nil
}

func (s *GatewayService) newClient(signer auth.Signer, apiKey *auth.APIKey) *polymarket.Client {
	opts := []polymarket.Option{
		polymarket.WithUseServerTime(true),
	}
	if s.config.Builder.ApiKey != "" {
		opts = append(opts, polymarket.WithBuilderAttribution(
			s.config.Builder.ApiKey,
			s.config.Builder.ApiSecret,
			s.config.Builder.ApiPassphrase,
		))
	}
	client := polymarket.NewClient(opts...)
	if signer != nil && apiKey != nil {
		client = client.WithAuth(signer, apiKey)
	}
	return client
}

func (s *GatewayService) getEIP1271Verifier() (*EIP1271Verifier, error) {
	if s.rpcURL == "" {
		return nil, fmt.Errorf("rpc url not configured")
	}
	if s.eip1271 == nil {
		ttl := time.Duration(s.config.Chain.EIP1271CacheSeconds) * time.Second
		timeout := time.Duration(s.config.Chain.EIP1271TimeoutMs) * time.Millisecond
		s.eip1271 = NewEIP1271Verifier(s.rpcURL, ttl, timeout, s.config.Chain.EIP1271Retries)
	}
	return s.eip1271, nil
}

func (s *GatewayService) buildSignable(ctx context.Context, client *polymarket.Client, signer auth.Signer, req OrderRequest) (*clobtypes.SignableOrder, error) {
	orderType := parseOrderType(req.OrderType)
	builder := clob.NewOrderBuilder(client.CLOB, signer).
		TokenID(req.TokenID).
		Price(req.Price).
		Size(req.Size).
		Side(req.Side).
		OrderType(orderType)
	if req.PostOnly != nil {
		builder.PostOnly(*req.PostOnly)
	}
	if req.Expiration > 0 {
		builder.ExpirationUnix(req.Expiration)
	}
	signable, err := builder.BuildSignableWithContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.SignatureType != nil {
		sigType := *req.SignatureType
		signable.Order.SignatureType = &sigType
		chainID := signer.ChainID().Int64()
		switch auth.SignatureType(sigType) {
		case auth.SignatureProxy:
			proxy, err := auth.DeriveProxyWalletForChain(signer.Address(), chainID)
			if err != nil && chainID == 0 {
				proxy, err = auth.DeriveProxyWallet(signer.Address())
			}
			if err != nil {
				return nil, fmt.Errorf("failed to derive proxy wallet: %w", err)
			}
			signable.Order.Maker = proxy
		case auth.SignatureGnosisSafe:
			safe, err := auth.DeriveSafeWalletForChain(signer.Address(), chainID)
			if err != nil && chainID == 0 {
				safe, err = auth.DeriveSafeWallet(signer.Address())
			}
			if err != nil {
				return nil, fmt.Errorf("failed to derive safe wallet: %w", err)
			}
			signable.Order.Maker = safe
		default:
			signable.Order.Maker = signer.Address()
		}
	}
	return signable, nil
}

func parseOrderType(raw string) clobtypes.OrderType {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(clobtypes.OrderTypeGTD):
		return clobtypes.OrderTypeGTD
	case string(clobtypes.OrderTypeFAK):
		return clobtypes.OrderTypeFAK
	case string(clobtypes.OrderTypeFOK):
		return clobtypes.OrderTypeFOK
	default:
		return clobtypes.OrderTypeGTC
	}
}

func (s *GatewayService) checkMaxSlippage(ctx context.Context, client *polymarket.Client, tenant *model.Tenant, req OrderRequest) error {
	if tenant.Risk.MaxSlippage <= 0 {
		return nil
	}
	book, err := client.CLOB.OrderBook(ctx, &clobtypes.BookRequest{TokenID: req.TokenID})
	if err != nil {
		return fmt.Errorf("failed to fetch order book for slippage check: %w", err)
	}
	price := decimal.NewFromFloat(req.Price)
	slippage := decimal.NewFromFloat(tenant.Risk.MaxSlippage)
	one := decimal.NewFromInt(1)

	switch strings.ToUpper(req.Side) {
	case "BUY":
		if len(book.Asks) == 0 {
			return fmt.Errorf("order book empty for slippage check")
		}
		bestAsk, err := decimal.NewFromString(book.Asks[0].Price)
		if err != nil {
			return fmt.Errorf("invalid ask price for slippage check")
		}
		maxAllowed := bestAsk.Mul(one.Add(slippage))
		if price.GreaterThan(maxAllowed) {
			return fmt.Errorf("risk reject: price %.4f exceeds max slippage", req.Price)
		}
	case "SELL":
		if len(book.Bids) == 0 {
			return fmt.Errorf("order book empty for slippage check")
		}
		bestBid, err := decimal.NewFromString(book.Bids[0].Price)
		if err != nil {
			return fmt.Errorf("invalid bid price for slippage check")
		}
		minAllowed := bestBid.Mul(one.Sub(slippage))
		if price.LessThan(minAllowed) {
			return fmt.Errorf("risk reject: price %.4f exceeds max slippage", req.Price)
		}
	}
	return nil
}

func resolveAPIKey(tenant *model.Tenant, req OrderRequest) (*auth.APIKey, error) {
	if req.L2 != nil && req.L2.APIKey != "" && req.L2.APISecret != "" && req.L2.APIPassphrase != "" {
		return &auth.APIKey{
			Key:        req.L2.APIKey,
			Secret:     req.L2.APISecret,
			Passphrase: req.L2.APIPassphrase,
		}, nil
	}
	if tenant.Creds.L2ApiKey == "" || tenant.Creds.L2ApiSecret == "" || tenant.Creds.L2ApiPassphrase == "" {
		return nil, fmt.Errorf("missing L2 api credentials")
	}
	return &auth.APIKey{
		Key:        tenant.Creds.L2ApiKey,
		Secret:     tenant.Creds.L2ApiSecret,
		Passphrase: tenant.Creds.L2ApiPassphrase,
	}, nil
}

func tenantAllowsSigner(tenant *model.Tenant, signer string) bool {
	if len(tenant.AllowedSigners) == 0 {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(signer))
	for _, allowed := range tenant.AllowedSigners {
		if strings.ToLower(strings.TrimSpace(allowed)) == normalized {
			return true
		}
	}
	return false
}

func requestFromOrder(signable *clobtypes.SignableOrder) OrderRequest {
	order := signable.Order
	price := 0.0
	size := 0.0
	tokenID := ""
	if order != nil {
		if order.TokenID.Int != nil {
			tokenID = order.TokenID.Int.String()
		}
		maker := decimal.NewFromInt(0)
		taker := decimal.NewFromInt(0)
		if order.MakerAmount.BigInt() != nil {
			maker = order.MakerAmount
		}
		if order.TakerAmount.BigInt() != nil {
			taker = order.TakerAmount
		}
		switch strings.ToUpper(order.Side) {
		case "BUY":
			if !taker.IsZero() {
				size = taker.InexactFloat64()
				price = maker.Div(taker).InexactFloat64()
			}
		case "SELL":
			if !maker.IsZero() {
				size = maker.InexactFloat64()
				price = taker.Div(maker).InexactFloat64()
			}
		}
	}
	return OrderRequest{
		TokenID: tokenID,
		Price:   price,
		Size:    size,
		Side:    order.Side,
	}
}
