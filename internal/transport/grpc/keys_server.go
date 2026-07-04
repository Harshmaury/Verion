package grpc

import (
	"context"

	"github.com/Harshmaury/verion/internal/identity"
	verionv1 "github.com/Harshmaury/verion/api/gen/go/verion/v1"
)

// keyServer implements verionv1.KeyServiceServer.
type keyServer struct {
	verionv1.UnimplementedKeyServiceServer
	keySvc identity.KeyService
}

// Compile-time assertion.
var _ verionv1.KeyServiceServer = (*keyServer)(nil)

func newKeyServer(k identity.KeyService) *keyServer {
	return &keyServer{keySvc: k}
}

func (s *keyServer) GenerateKey(ctx context.Context, req *verionv1.GenerateKeyRequest) (*verionv1.GenerateKeyResponse, error) {
	input := identity.GenerateKeyInput{
		TenantID:   req.TenantId,
		IdentityID: req.IdentityId,
		KeyType:    keyTypeFromProto(req.KeyType),
		Purpose:    keyPurposeFromProto(req.Purpose),
	}
	if req.ActorId != "" {
		input.ActorID = &req.ActorId
	}

	key, err := s.keySvc.GenerateKey(ctx, input)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.GenerateKeyResponse{Key: identityKeyToProto(key)}, nil
}

func (s *keyServer) GetKey(ctx context.Context, req *verionv1.GetKeyRequest) (*verionv1.GetKeyResponse, error) {
	key, err := s.keySvc.GetKey(ctx, req.TenantId, req.KeyId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.GetKeyResponse{Key: identityKeyToProto(key)}, nil
}

func (s *keyServer) ListKeys(ctx context.Context, req *verionv1.ListKeysRequest) (*verionv1.ListKeysResponse, error) {
	keys, err := s.keySvc.ListKeys(ctx, req.TenantId, req.IdentityId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	protos := make([]*verionv1.IdentityKey, len(keys))
	for i, k := range keys {
		protos[i] = identityKeyToProto(k)
	}
	return &verionv1.ListKeysResponse{Keys: protos}, nil
}

func (s *keyServer) RotateKey(ctx context.Context, req *verionv1.RotateKeyRequest) (*verionv1.RotateKeyResponse, error) {
	newKey, err := s.keySvc.RotateKey(ctx, req.TenantId, req.IdentityId, req.KeyId, req.ActorId)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.RotateKeyResponse{NewKey: identityKeyToProto(newKey)}, nil
}

func (s *keyServer) RevokeKey(ctx context.Context, req *verionv1.RevokeKeyRequest) (*verionv1.RevokeKeyResponse, error) {
	if err := s.keySvc.RevokeKey(ctx, req.TenantId, req.KeyId, req.ActorId); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.RevokeKeyResponse{}, nil
}

// ── Enum mappers: proto → domain (key-specific) ───────────────────────────────

func keyTypeFromProto(t verionv1.KeyType) identity.KeyType {
	switch t {
	case verionv1.KeyType_KEY_TYPE_ED25519:
		return identity.KeyTypeEd25519
	case verionv1.KeyType_KEY_TYPE_ECDSA_P256:
		return identity.KeyTypeECDSAP256
	case verionv1.KeyType_KEY_TYPE_ECDSA_P384:
		return identity.KeyTypeECDSAP384
	case verionv1.KeyType_KEY_TYPE_RSA_4096:
		return identity.KeyTypeRSA4096
	default:
		return identity.KeyTypeEd25519
	}
}

func keyPurposeFromProto(p verionv1.KeyPurpose) identity.KeyPurpose {
	switch p {
	case verionv1.KeyPurpose_KEY_PURPOSE_SIGNING:
		return identity.KeyPurposeSigning
	case verionv1.KeyPurpose_KEY_PURPOSE_ENCRYPTION:
		return identity.KeyPurposeEncryption
	case verionv1.KeyPurpose_KEY_PURPOSE_AUTHENTICATION:
		return identity.KeyPurposeAuthentication
	case verionv1.KeyPurpose_KEY_PURPOSE_RECOVERY:
		return identity.KeyPurposeRecovery
	default:
		return identity.KeyPurposeSigning
	}
}
