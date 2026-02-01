package market

type Provider interface {
	Subscribe(tokenIDs []string)
	GetBook(tokenID string) *Orderbook
	Start()
	Stop()
}
