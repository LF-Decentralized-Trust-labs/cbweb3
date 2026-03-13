package kmsprovider

import "context"

type KeyInfo struct {
	KeyID   string
	Address string
}

type Provider interface {
	Name() string
	CreateKey(ctx context.Context, userID string) (KeyInfo, error)
	GetByUser(ctx context.Context, userID string) (KeyInfo, bool)
	SignDigest(ctx context.Context, keyID, digestHex string) (string, error)
}
