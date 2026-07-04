package grpc

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Harshmaury/verion/internal/identity"
	verionv1 "github.com/Harshmaury/verion/api/gen/go/verion/v1"
)

// identityServer implements verionv1.IdentityServiceServer.
type identityServer struct {
	verionv1.UnimplementedIdentityServiceServer
	identitySvc identity.IdentityService
	tenantSvc   identity.TenantService
}

// Compile-time assertion.
var _ verionv1.IdentityServiceServer = (*identityServer)(nil)

func newIdentityServer(i identity.IdentityService, t identity.TenantService) *identityServer {
	return &identityServer{identitySvc: i, tenantSvc: t}
}

// ── Tenant handlers ───────────────────────────────────────────────────────────

func (s *identityServer) CreateTenant(ctx context.Context, req *verionv1.CreateTenantRequest) (*verionv1.CreateTenantResponse, error) {
	tenant, err := s.tenantSvc.CreateTenant(ctx, identity.CreateTenantInput{
		Name:       req.Name,
		Slug:       req.Slug,
		Tier:       tenantTierFromProto(req.Tier),
		DataRegion: req.DataRegion,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.CreateTenantResponse{Tenant: tenantToProto(tenant)}, nil
}

func (s *identityServer) GetTenant(ctx context.Context, req *verionv1.GetTenantRequest) (*verionv1.GetTenantResponse, error) {
	tenant, err := s.tenantSvc.GetTenant(ctx, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.GetTenantResponse{Tenant: tenantToProto(tenant)}, nil
}

func (s *identityServer) SuspendTenant(ctx context.Context, req *verionv1.SuspendTenantRequest) (*verionv1.SuspendTenantResponse, error) {
	if err := s.tenantSvc.SuspendTenant(ctx, req.Id); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.SuspendTenantResponse{}, nil
}

func (s *identityServer) ActivateTenant(ctx context.Context, req *verionv1.ActivateTenantRequest) (*verionv1.ActivateTenantResponse, error) {
	if err := s.tenantSvc.ActivateTenant(ctx, req.Id); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.ActivateTenantResponse{}, nil
}

// ── Identity handlers ─────────────────────────────────────────────────────────

func (s *identityServer) CreateIdentity(ctx context.Context, req *verionv1.CreateIdentityRequest) (*verionv1.CreateIdentityResponse, error) {
	input := identity.CreateIdentityInput{
		TenantID:    req.TenantId,
		Type:        identityTypeFromProto(req.Type),
		DisplayName: req.DisplayName,
		Handle:      req.Handle,
		Attributes:  stringMapToAny(req.Attributes),
	}
	if req.CreatedBy != "" {
		input.CreatedBy = &req.CreatedBy
	}

	result, err := s.identitySvc.CreateIdentity(ctx, input)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &verionv1.CreateIdentityResponse{
		Identity: identityToProto(result.Identity),
	}
	if len(result.Keys) > 0 {
		resp.Key = identityKeyToProto(result.Keys[0])
	}
	return resp, nil
}

func (s *identityServer) GetIdentity(ctx context.Context, req *verionv1.GetIdentityRequest) (*verionv1.GetIdentityResponse, error) {
	id, err := s.identitySvc.GetIdentity(ctx, req.TenantId, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.GetIdentityResponse{Identity: identityToProto(id)}, nil
}

func (s *identityServer) GetIdentityByHandle(ctx context.Context, req *verionv1.GetIdentityByHandleRequest) (*verionv1.GetIdentityResponse, error) {
	id, err := s.identitySvc.GetIdentityByHandle(ctx, req.TenantId, req.Handle)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.GetIdentityResponse{Identity: identityToProto(id)}, nil
}

func (s *identityServer) ListIdentities(ctx context.Context, req *verionv1.ListIdentitiesRequest) (*verionv1.ListIdentitiesResponse, error) {
	filter := identity.IdentityFilter{
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}
	if req.Type != "" {
		t := identity.IdentityType(req.Type)
		filter.Type = &t
	}
	if req.Status != "" {
		st := identity.IdentityStatus(req.Status)
		filter.Status = &st
	}

	identities, err := s.identitySvc.ListIdentities(ctx, req.TenantId, filter)
	if err != nil {
		return nil, toGRPCError(err)
	}

	protos := make([]*verionv1.Identity, len(identities))
	for i, id := range identities {
		protos[i] = identityToProto(id)
	}
	return &verionv1.ListIdentitiesResponse{
		Identities: protos,
		Total:      int64(len(protos)),
	}, nil
}

func (s *identityServer) UpdateIdentity(ctx context.Context, req *verionv1.UpdateIdentityRequest) (*verionv1.UpdateIdentityResponse, error) {
	input := identity.UpdateIdentityInput{
		TenantID:    req.TenantId,
		ID:          req.Id,
		DisplayName: req.DisplayName,
		Attributes:  stringMapToAny(req.Attributes),
		Version:     req.Version,
	}
	if req.ActorId != "" {
		input.ActorID = &req.ActorId
	}

	updated, err := s.identitySvc.UpdateIdentity(ctx, input)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.UpdateIdentityResponse{Identity: identityToProto(updated)}, nil
}

func (s *identityServer) SuspendIdentity(ctx context.Context, req *verionv1.SuspendIdentityRequest) (*verionv1.SuspendIdentityResponse, error) {
	if err := s.identitySvc.SuspendIdentity(ctx, req.TenantId, req.Id, req.ActorId); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.SuspendIdentityResponse{}, nil
}

func (s *identityServer) ReactivateIdentity(ctx context.Context, req *verionv1.ReactivateIdentityRequest) (*verionv1.ReactivateIdentityResponse, error) {
	if err := s.identitySvc.ReactivateIdentity(ctx, req.TenantId, req.Id, req.ActorId); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.ReactivateIdentityResponse{}, nil
}

func (s *identityServer) DeactivateIdentity(ctx context.Context, req *verionv1.DeactivateIdentityRequest) (*verionv1.DeactivateIdentityResponse, error) {
	if err := s.identitySvc.DeactivateIdentity(ctx, req.TenantId, req.Id, req.ActorId); err != nil {
		return nil, toGRPCError(err)
	}
	return &verionv1.DeactivateIdentityResponse{}, nil
}

// ── Domain → Proto mappers ────────────────────────────────────────────────────

func tenantToProto(t *identity.Tenant) *verionv1.Tenant {
	return &verionv1.Tenant{
		Id:         t.ID,
		Name:       t.Name,
		Slug:       t.Slug,
		Tier:       tenantTierToProto(t.Tier),
		Status:     string(t.Status),
		DataRegion: t.DataRegion,
		CreatedAt:  timestamppb.New(t.CreatedAt),
		UpdatedAt:  timestamppb.New(t.UpdatedAt),
	}
}

func identityToProto(i *identity.Identity) *verionv1.Identity {
	p := &verionv1.Identity{
		Id:          i.ID,
		TenantId:    i.TenantID,
		Type:        identityTypeToProto(i.Type),
		DisplayName: i.DisplayName,
		Handle:      i.Handle,
		Status:      identityStatusToProto(i.Status),
		Version:     i.Version,
		CreatedAt:   timestamppb.New(i.CreatedAt),
		UpdatedAt:   timestamppb.New(i.UpdatedAt),
		Attributes:  anyMapToString(i.DecryptedAttributes),
	}
	if i.PrimaryKeyID != nil {
		p.PrimaryKeyId = *i.PrimaryKeyID
	}
	return p
}

func identityKeyToProto(k *identity.IdentityKey) *verionv1.IdentityKey {
	p := &verionv1.IdentityKey{
		Id:          k.ID,
		IdentityId:  k.IdentityID,
		TenantId:    k.TenantID,
		KeyType:     keyTypeToProto(k.KeyType),
		Purpose:     keyPurposeToProto(k.Purpose),
		Algorithm:   k.Algorithm,
		PublicKey:   k.PublicKey,
		Fingerprint: k.Fingerprint,
		Status:      keyStatusToProto(k.Status),
		CreatedAt:   timestamppb.New(k.CreatedAt),
		ValidFrom:   timestamppb.New(k.ValidFrom),
	}
	if k.ValidUntil != nil {
		p.ValidUntil = timestamppb.New(*k.ValidUntil)
	}
	return p
}

// ── Enum mappers: proto → domain ──────────────────────────────────────────────

func identityTypeFromProto(t verionv1.IdentityType) identity.IdentityType {
	switch t {
	case verionv1.IdentityType_IDENTITY_TYPE_HUMAN:
		return identity.IdentityTypeHuman
	case verionv1.IdentityType_IDENTITY_TYPE_ORG:
		return identity.IdentityTypeOrg
	case verionv1.IdentityType_IDENTITY_TYPE_DEVICE:
		return identity.IdentityTypeDevice
	case verionv1.IdentityType_IDENTITY_TYPE_SERVICE:
		return identity.IdentityTypeService
	case verionv1.IdentityType_IDENTITY_TYPE_MACHINE:
		return identity.IdentityTypeMachine
	case verionv1.IdentityType_IDENTITY_TYPE_AI_AGENT:
		return identity.IdentityTypeAIAgent
	default:
		return identity.IdentityTypeHuman
	}
}

func tenantTierFromProto(t verionv1.TenantTier) identity.TenantTier {
	switch t {
	case verionv1.TenantTier_TENANT_TIER_PROFESSIONAL:
		return identity.TenantTierProfessional
	case verionv1.TenantTier_TENANT_TIER_ENTERPRISE:
		return identity.TenantTierEnterprise
	default:
		return identity.TenantTierStandard
	}
}

// ── Enum mappers: domain → proto ──────────────────────────────────────────────

func identityTypeToProto(t identity.IdentityType) verionv1.IdentityType {
	switch t {
	case identity.IdentityTypeHuman:
		return verionv1.IdentityType_IDENTITY_TYPE_HUMAN
	case identity.IdentityTypeOrg:
		return verionv1.IdentityType_IDENTITY_TYPE_ORG
	case identity.IdentityTypeDevice:
		return verionv1.IdentityType_IDENTITY_TYPE_DEVICE
	case identity.IdentityTypeService:
		return verionv1.IdentityType_IDENTITY_TYPE_SERVICE
	case identity.IdentityTypeMachine:
		return verionv1.IdentityType_IDENTITY_TYPE_MACHINE
	case identity.IdentityTypeAIAgent:
		return verionv1.IdentityType_IDENTITY_TYPE_AI_AGENT
	default:
		return verionv1.IdentityType_IDENTITY_TYPE_UNSPECIFIED
	}
}

func identityStatusToProto(s identity.IdentityStatus) verionv1.IdentityStatus {
	switch s {
	case identity.IdentityStatusPending:
		return verionv1.IdentityStatus_IDENTITY_STATUS_PENDING
	case identity.IdentityStatusActive:
		return verionv1.IdentityStatus_IDENTITY_STATUS_ACTIVE
	case identity.IdentityStatusSuspended:
		return verionv1.IdentityStatus_IDENTITY_STATUS_SUSPENDED
	case identity.IdentityStatusDeactivated:
		return verionv1.IdentityStatus_IDENTITY_STATUS_DEACTIVATED
	case identity.IdentityStatusArchived:
		return verionv1.IdentityStatus_IDENTITY_STATUS_ARCHIVED
	default:
		return verionv1.IdentityStatus_IDENTITY_STATUS_UNSPECIFIED
	}
}

func tenantTierToProto(t identity.TenantTier) verionv1.TenantTier {
	switch t {
	case identity.TenantTierStandard:
		return verionv1.TenantTier_TENANT_TIER_STANDARD
	case identity.TenantTierProfessional:
		return verionv1.TenantTier_TENANT_TIER_PROFESSIONAL
	case identity.TenantTierEnterprise:
		return verionv1.TenantTier_TENANT_TIER_ENTERPRISE
	default:
		return verionv1.TenantTier_TENANT_TIER_UNSPECIFIED
	}
}

func keyTypeToProto(t identity.KeyType) verionv1.KeyType {
	switch t {
	case identity.KeyTypeEd25519:
		return verionv1.KeyType_KEY_TYPE_ED25519
	case identity.KeyTypeECDSAP256:
		return verionv1.KeyType_KEY_TYPE_ECDSA_P256
	default:
		return verionv1.KeyType_KEY_TYPE_UNSPECIFIED
	}
}

func keyPurposeToProto(p identity.KeyPurpose) verionv1.KeyPurpose {
	switch p {
	case identity.KeyPurposeSigning:
		return verionv1.KeyPurpose_KEY_PURPOSE_SIGNING
	case identity.KeyPurposeEncryption:
		return verionv1.KeyPurpose_KEY_PURPOSE_ENCRYPTION
	case identity.KeyPurposeAuthentication:
		return verionv1.KeyPurpose_KEY_PURPOSE_AUTHENTICATION
	case identity.KeyPurposeRecovery:
		return verionv1.KeyPurpose_KEY_PURPOSE_RECOVERY
	default:
		return verionv1.KeyPurpose_KEY_PURPOSE_UNSPECIFIED
	}
}

func keyStatusToProto(s identity.KeyStatus) verionv1.KeyStatus {
	switch s {
	case identity.KeyStatusActive:
		return verionv1.KeyStatus_KEY_STATUS_ACTIVE
	case identity.KeyStatusRotated:
		return verionv1.KeyStatus_KEY_STATUS_ROTATED
	case identity.KeyStatusRevoked:
		return verionv1.KeyStatus_KEY_STATUS_REVOKED
	case identity.KeyStatusCompromised:
		return verionv1.KeyStatus_KEY_STATUS_COMPROMISED
	default:
		return verionv1.KeyStatus_KEY_STATUS_UNSPECIFIED
	}
}

// ── Attribute conversion helpers ──────────────────────────────────────────────

// stringMapToAny converts map[string]string from proto to map[string]any for service layer.
func stringMapToAny(m map[string]string) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// anyMapToString converts map[string]any from service layer to map[string]string for proto.
// Non-string values are formatted with fmt.Sprintf.
func anyMapToString(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			out[k] = val
		default:
			out[k] = fmt.Sprintf("%v", val)
		}
	}
	return out
}
