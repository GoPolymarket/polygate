package model

import "github.com/GoPolymarket/polymarket-go-sdk/pkg/clob/clobtypes"

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

// CancelOrderInput defines parameters for cancelling a single order
type CancelOrderInput struct {
	ID string `json:"id" binding:"required"`
}
