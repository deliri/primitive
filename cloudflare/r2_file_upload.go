package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2FileUpload is one sealed file retained create-only in R2. CredentialFile
// names a private static R2 key file; Expected is the sealed file's exact
// SHA-256 and extent; SignedAt and Expires bound both signed operations.
type R2FileUpload struct {
	CredentialFile core.AbsolutePath
	Account        AccountID
	Jurisdiction   R2Jurisdiction
	Bucket         R2Bucket
	Key            R2Key
	Source         io.Reader
	Expected       filestore.ContentIndexEntry
	ContentType    core.HTTPMediaType
	SignedAt       temporal.Instant
	Expires        temporal.Duration
	Policy         exchange.StreamPolicy
}

func (r R2FileUpload) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrCloudflareContract
	}
	if err := errors.Join(r.CredentialFile.Validate(), r.Account.Validate(), r.Jurisdiction.Validate(), r.Bucket.Validate(), r.Key.Validate(), r.Expected.Validate(), validatePolicy(r.Policy)); err != nil {
		return contractError(err)
	}
	if !r.ContentType.IsZero() {
		if err := r.ContentType.Validate(); err != nil {
			return contractError(err)
		}
	}
	if r.Expected.Extent.Uint64() > core.CloudflareR2SingleUploadMaximumBytes {
		return core.ErrCloudflareBinding
	}
	return nil
}

// R2VerifiedObject proves that R2 returned exactly the sealed bytes for one
// object. Its zero value proves nothing.
type R2VerifiedObject struct {
	bucket R2Bucket
	key    R2Key
	digest core.SHA256Digest
	extent core.ByteLength
}

func (o R2VerifiedObject) Validate() error {
	return errors.Join(o.bucket.Validate(), o.key.Validate(), o.digest.Validate(), o.extent.Validate())
}
func (o R2VerifiedObject) Bucket() R2Bucket          { return o.bucket }
func (o R2VerifiedObject) Key() R2Key                { return o.key }
func (o R2VerifiedObject) Digest() core.SHA256Digest { return o.digest }
func (o R2VerifiedObject) Extent() core.ByteLength   { return o.extent }

// r2CredentialFile is the strict on-disk shape of a static R2 key: exactly
// these two members; any other member, token or profile is refused.
type r2CredentialFile struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func (f r2CredentialFile) Validate() error {
	if f.AccessKeyID == "" || f.SecretAccessKey == "" {
		return core.ErrCloudflareAuthentication
	}
	return nil
}

// UploadR2File streams the sealed source to R2 create-only with its exact
// extent, then reads the object back through a signed GET and proves its
// SHA-256 and extent equal the expected ones. Any failure returns zero proof
// with the native cause; nothing is retried. The credentials are destroyed on
// every return; the decoded JSON text itself is not zeroable in Go.
func UploadR2File(ctx context.Context, client R2Client, request R2FileUpload) (R2VerifiedObject, error) {
	if err := errors.Join(contextstate.Validate(ctx), client.Validate(), request.Validate()); err != nil {
		return R2VerifiedObject{}, err
	}
	credentials, err := readR2Credentials(ctx, request.CredentialFile)
	if err != nil {
		return R2VerifiedObject{}, err
	}
	server, err := NewR2Server(credentials, request.Account, request.Jurisdiction)
	closeErr := credentials.Close()
	if err != nil || closeErr != nil {
		return R2VerifiedObject{}, errors.Join(err, closeErr)
	}
	verified, err := uploadAndVerify(ctx, r2FileTransfer{client: client, server: server, request: request})
	if closeErr := server.Close(); err != nil || closeErr != nil {
		return R2VerifiedObject{}, errors.Join(err, closeErr)
	}
	return verified, nil
}

// readR2Credentials reads a private credential file: one whose permissions
// grant nothing to group or others. The read buffer is cleared on return.
func readR2Credentials(ctx context.Context, path core.AbsolutePath) (R2Credentials, error) {
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return R2Credentials{}, errors.Join(core.ErrCloudflareAuthentication, err)
	}
	credentials, err := readPrivateR2Credentials(ctx, location)
	if closeErr := location.Root.Close(); err != nil || closeErr != nil {
		return R2Credentials{}, errors.Join(core.ErrCloudflareAuthentication, err, closeErr)
	}
	return credentials, nil
}

func readPrivateR2Credentials(ctx context.Context, location filestore.Location) (R2Credentials, error) {
	observed, err := filestore.InspectEntry(ctx, location)
	if err != nil {
		return R2Credentials{}, err
	}
	permissions, err := observed.Permissions()
	if err != nil {
		return R2Credentials{}, err
	}
	mode, err := permissions.FileMode()
	if err != nil {
		return R2Credentials{}, err
	}
	if mode.Perm()&0o077 != 0 {
		return R2Credentials{}, core.ErrCloudflareAuthentication
	}
	var buffer bytes.Buffer
	defer func() { clear(buffer.Bytes()) }()
	if _, err := filestore.Read(ctx, filestore.ReadRequest{Destination: &buffer, Location: location}); err != nil {
		return R2Credentials{}, err
	}
	return decodeR2Credentials(buffer.Bytes())
}

// decodeR2Credentials admits exactly the strict two-member document.
func decodeR2Credentials(data []byte) (R2Credentials, error) {
	document, err := jsonio.DecodeOwned[r2CredentialFile](data, jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly})
	if err != nil {
		return R2Credentials{}, errors.Join(core.ErrCloudflareAuthentication, err)
	}
	return ParseR2Credentials([]byte(document.AccessKeyID), []byte(document.SecretAccessKey))
}

// r2FileTransfer lends one signing server and client to one sealed file.
type r2FileTransfer struct {
	client  R2Client
	server  R2Server
	request R2FileUpload
}

func uploadAndVerify(ctx context.Context, transfer r2FileTransfer) (R2VerifiedObject, error) {
	request := transfer.request
	put, err := transfer.server.Presign(ctx, R2PresignRequest{Bucket: request.Bucket, Key: request.Key, ContentType: request.ContentType, SignedAt: request.SignedAt, Expires: request.Expires, Method: exchange.MethodPut, Conditions: R2WriteConditions{CreateOnly: true}})
	if err != nil {
		return R2VerifiedObject{}, err
	}
	if _, err := transfer.client.Put(ctx, R2WriteRequest{Source: request.Source, Grant: put, Bytes: request.Expected.Extent}, request.Policy); err != nil {
		return R2VerifiedObject{}, err
	}
	get, err := transfer.server.Presign(ctx, R2PresignRequest{Bucket: request.Bucket, Key: request.Key, SignedAt: request.SignedAt, Expires: request.Expires, Method: exchange.MethodGet})
	if err != nil {
		return R2VerifiedObject{}, err
	}
	digest := core.NewDigestWriter()
	if _, err := transfer.client.Read(ctx, get, digest, request.Policy); err != nil {
		return R2VerifiedObject{}, err
	}
	sha, extent, err := digest.Seal()
	if err != nil {
		return R2VerifiedObject{}, err
	}
	if sha != request.Expected.Digest || extent.Uint64() != request.Expected.Extent.Uint64() {
		return R2VerifiedObject{}, core.ErrCloudflareVerification
	}
	return R2VerifiedObject{bucket: request.Bucket, key: request.Key, digest: sha, extent: request.Expected.Extent}, nil
}
