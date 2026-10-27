package gcsobjects

import (
	"context"
	"errors"
	"slices"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
)

// GCSObjectPublicReadRequest grants read access to one immutable generation.
// It never changes bucket IAM, default object ACLs, or another generation.
type GCSObjectPublicReadRequest struct {
	Bucket     GCSBucket
	Name       GCSObjectName
	Generation GCSGeneration
}

func (r GCSObjectPublicReadRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Name.Validate(), r.Generation.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// GCSObjectPublicReadGrant is sealed provider confirmation of the exact grant.
type GCSObjectPublicReadGrant struct {
	metadata GCSObjectMetadata
	changed  bool
	set      bool
}

func (g GCSObjectPublicReadGrant) Validate() error {
	if !g.set {
		return core.ErrObjectStoreContract
	}
	return g.metadata.Validate()
}

func (g GCSObjectPublicReadGrant) Metadata() (GCSObjectMetadata, error) {
	if err := g.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return g.metadata, nil
}

func (g GCSObjectPublicReadGrant) Changed() (bool, error) {
	if err := g.Validate(); err != nil {
		return false, err
	}
	return g.changed, nil
}

// GrantGCSObjectPublicRead preserves every existing ACL entry and conditionally
// adds only allUsers READER. The official object's metadata update binds both
// generation and metageneration; ACLHandle.Set does not bind a generation.
// A failed update may have reached the provider, so errors carry no grant.
func GrantGCSObjectPublicRead(ctx context.Context, client *GCSClient, request GCSObjectPublicReadRequest) (GCSObjectPublicReadGrant, error) {
	if err := errors.Join(request.Validate(), validateGCSCall(ctx, client)); err != nil {
		return GCSObjectPublicReadGrant{}, err
	}
	generation, err := request.Generation.Int64()
	if err != nil {
		return GCSObjectPublicReadGrant{}, err
	}
	object := client.client.Bucket(request.Bucket.String()).Object(request.Name.String()).Generation(generation)
	before, err := object.Attrs(ctx)
	if err != nil {
		return GCSObjectPublicReadGrant{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := publicReadMetadata(request, before)
	if err != nil {
		return GCSObjectPublicReadGrant{}, err
	}
	// An absent ACL is not authority to replace an unobserved policy. Unknown
	// or contradictory provider roles likewise cannot become a smaller ACL.
	if err := validateGCSPublicationACL(before.ACL); err != nil {
		return GCSObjectPublicReadGrant{}, err
	}
	if gcsACLHasPublicRead(before.ACL) {
		return GCSObjectPublicReadGrant{metadata: metadata, set: true}, nil
	}
	acl := append(slices.Clone(before.ACL), storage.ACLRule{Entity: storage.AllUsers, Role: storage.RoleReader})
	after, err := object.If(storage.Conditions{MetagenerationMatch: before.Metageneration}).Update(ctx, storage.ObjectAttrsToUpdate{ACL: acl})
	if err != nil {
		return GCSObjectPublicReadGrant{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	metadata, err = publicReadMetadata(request, after)
	if err != nil || after.Metageneration <= before.Metageneration || !sameGCSPublicationACL(acl, after.ACL) {
		return GCSObjectPublicReadGrant{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination, core.ErrObjectStoreConflict, err)
	}
	return GCSObjectPublicReadGrant{metadata: metadata, changed: true, set: true}, nil
}

func publicReadMetadata(request GCSObjectPublicReadRequest, attributes *storage.ObjectAttrs) (GCSObjectMetadata, error) {
	metadata, err := metadataFromGCSAttrs(attributes)
	if err != nil || attributes.Metageneration <= 0 {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	if metadata.Bucket() != request.Bucket || metadata.Name() != request.Name || metadata.Generation() != request.Generation {
		return GCSObjectMetadata{}, core.ErrObjectStoreIntegrity
	}
	return metadata, nil
}

func validateGCSPublicationACL(acl []storage.ACLRule) error {
	if len(acl) == 0 {
		return core.ErrObjectStoreContract
	}
	for index, rule := range acl {
		if rule.Entity == "" || (rule.Entity == storage.AllUsers && rule.Role != storage.RoleReader) || (rule.Role != storage.RoleReader && rule.Role != storage.RoleWriter && rule.Role != storage.RoleOwner) {
			return core.ErrObjectStoreContract
		}
		for _, previous := range acl[:index] {
			if previous.Entity == rule.Entity {
				return core.ErrObjectStoreContract
			}
		}
	}
	return nil
}

func gcsACLHasPublicRead(acl []storage.ACLRule) bool {
	return slices.ContainsFunc(acl, func(rule storage.ACLRule) bool {
		return rule.Entity == storage.AllUsers && rule.Role == storage.RoleReader
	})
}

func sameGCSPublicationACL(want, got []storage.ACLRule) bool {
	if len(want) != len(got) {
		return false
	}
	for _, rule := range want {
		if !slices.ContainsFunc(got, func(candidate storage.ACLRule) bool {
			return rule.Entity == candidate.Entity && rule.Role == candidate.Role
		}) {
			return false
		}
	}
	return true
}
