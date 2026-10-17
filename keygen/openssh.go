package keygen

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// PEMLabel owns a PEM envelope label, rather than a key algorithm or policy.
type PEMLabel string

const OpenSSHPrivateKeyLabel PEMLabel = "OPENSSH PRIVATE KEY"

func (p PEMLabel) Validate() error {
	if len(p) == 0 || p[0] == ' ' || p[len(p)-1] == ' ' {
		return core.ErrKeygenContract
	}
	for _, value := range p {
		if (value < 'A' || value > 'Z') && (value < '0' || value > '9') && value != ' ' {
			return core.ErrKeygenContract
		}
	}
	return nil
}

func (p PEMLabel) String() string { return string(p) }

type OpenSSHReadRequest struct {
	Source          io.Reader
	AdditionalLabel PEMLabel
}

func (r OpenSSHReadRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrKeygenContract
	}
	if r.AdditionalLabel != "" {
		return r.AdditionalLabel.Validate()
	}
	return nil
}

type OpenSSHWriteRequest struct {
	Destination io.Writer
	Key         SigningKey
	Label       PEMLabel
	Comment     string
}

func (r OpenSSHWriteRequest) Validate() error {
	if core.WriterIsNil(r.Destination) || uint64(len(r.Comment))+uint64(openSSHPrivateFixed)+7 > math.MaxUint32 {
		return core.ErrKeygenContract
	}
	return errors.Join(r.Label.Validate(), r.Key.Validate())
}

const (
	openSSHMagic        = "openssh-key-v1\x00"
	openSSHAlgorithm    = "ssh-ed25519"
	openSSHCipher       = "none"
	openSSHPublicBytes  = ed25519.PublicKeySize
	openSSHPrivateBytes = ed25519.PrivateKeySize
	openSSHPublicExtent = 4 + len(openSSHAlgorithm) + 4 + openSSHPublicBytes
	openSSHPrivateFixed = 8 + 4 + len(openSSHAlgorithm) + 4 + openSSHPublicBytes + 4 + openSSHPrivateBytes + 4
)

type keyStream struct {
	ctx         context.Context
	source      io.Reader
	destination io.Writer
}

func (s keyStream) Read(p []byte) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, errors.Join(err, context.Cause(s.ctx))
	}
	n, err := s.source.Read(p)
	if canceled := s.ctx.Err(); canceled != nil {
		return 0, errors.Join(err, canceled, context.Cause(s.ctx))
	}
	return n, err
}

func (s keyStream) Write(p []byte) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, errors.Join(err, context.Cause(s.ctx))
	}
	n, err := s.destination.Write(p)
	if n < 0 || n > len(p) {
		return 0, errors.Join(core.ErrKeygenContract, err)
	}
	if n < len(p) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err
}

// ReadOpenSSH consumes one unencrypted Ed25519 key, then clean source exhaustion.
// Comments are discarded forward without retention. Secret storage and parser
// scratch have fixed extents; no total input quota is imposed. Native read,
// base64 and context failures withdraw the key and preserve their causes.
func ReadOpenSSH(ctx context.Context, request OpenSSHReadRequest) (result SigningKey, resultErr error) {
	if ctx == nil {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, core.ErrNilContext)
	}
	if err := errors.Join(ctx.Err(), context.Cause(ctx), request.Validate()); err != nil {
		return SigningKey{}, contractError(err)
	}
	source := bufio.NewReaderSize(keyStream{ctx: ctx, source: request.Source}, 4096)
	label, err := readKeyPEMHeader(source, request.AdditionalLabel)
	if err != nil {
		return SigningKey{}, contractError(err)
	}
	body := &keyPEMBody{source: source, footer: "-----END " + label.String() + "-----", lineStart: true}
	decoded := base64.NewDecoder(base64.StdEncoding.Strict(), body)
	key, err := readOpenSSHBinary(decoded)
	if err != nil {
		return SigningKey{}, contractError(err)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, key.Destroy())
		}
	}()
	var extra [1]byte
	n, err := io.ReadFull(decoded, extra[:])
	if n != 0 || err != io.EOF {
		return SigningKey{}, contractError(err)
	}
	// A padded base64 decoder may finish before requesting the PEM footer.
	// Drain framing separately and refuse any remaining encoded payload.
	n, err = body.Read(extra[:])
	if n != 0 || err != io.EOF || !body.finished {
		return SigningKey{}, contractError(err)
	}
	for {
		value, err := source.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return SigningKey{}, contractError(err)
		}
		if value != ' ' && value != '\t' && value != '\r' && value != '\n' {
			return SigningKey{}, core.ErrKeygenContract
		}
	}
	if err := ctx.Err(); err != nil {
		return SigningKey{}, contractError(errors.Join(err, context.Cause(ctx)))
	}
	return key, nil
}

func readKeyPEMHeader(source *bufio.Reader, additional PEMLabel) (PEMLabel, error) {
	first := "-----BEGIN " + OpenSSHPrivateKeyLabel.String() + "-----"
	second := "-----BEGIN " + additional.String() + "-----"
	a, b := true, additional != ""
	for index := 0; ; index++ {
		value, err := source.ReadByte()
		if err != nil {
			return "", err
		}
		if value == '\r' {
			value, err = source.ReadByte()
			if err != nil {
				return "", err
			}
			if value != '\n' {
				return "", core.ErrKeygenContract
			}
		}
		if value == '\n' {
			if a && index == len(first) {
				return OpenSSHPrivateKeyLabel, nil
			}
			if b && index == len(second) {
				return additional, nil
			}
			return "", core.ErrKeygenContract
		}
		a = a && index < len(first) && first[index] == value
		b = b && index < len(second) && second[index] == value
		if !a && !b {
			return "", core.ErrKeygenContract
		}
	}
}

type keyPEMBody struct {
	source    *bufio.Reader
	footer    string
	lineStart bool
	finished  bool
}

func (s *keyPEMBody) Read(p []byte) (n int, resultErr error) {
	if s.finished {
		return 0, io.EOF
	}
	for n < len(p) {
		value, err := s.source.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return n, err
		}
		if value == '-' && s.lineStart {
			for index := 1; index < len(s.footer); index++ {
				value, err := s.source.ReadByte()
				if err != nil {
					return n, err
				}
				if value != s.footer[index] {
					return n, core.ErrKeygenContract
				}
			}
			value, err := s.source.ReadByte()
			if value == '\r' && err == nil {
				value, err = s.source.ReadByte()
			}
			if err != io.EOF && (err != nil || value != '\n') {
				return n, errors.Join(core.ErrKeygenContract, err)
			}
			s.finished = true
			return n, io.EOF
		}
		if value == '\n' {
			s.lineStart = true
			continue
		}
		if value == '\r' || value == ' ' || value == '\t' {
			continue
		}
		s.lineStart = false
		p[n] = value
		n++
	}
	return n, nil
}

func readOpenSSHBinary(source io.Reader) (SigningKey, error) {
	var magic [len(openSSHMagic)]byte
	if _, err := io.ReadFull(source, magic[:]); err != nil {
		return SigningKey{}, err
	}
	if string(magic[:]) != openSSHMagic {
		return SigningKey{}, core.ErrKeygenContract
	}
	var name [len(openSSHCipher)]byte
	for range 2 {
		if err := readKeySSHField(source, name[:]); err != nil {
			return SigningKey{}, err
		}
		if string(name[:]) != openSSHCipher {
			return SigningKey{}, core.ErrKeygenContract
		}
	}
	options, err := readKeyUint32(source)
	if err != nil || options != 0 {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err)
	}
	count, err := readKeyUint32(source)
	if err != nil || count != 1 {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err)
	}
	var outer [openSSHPublicExtent]byte
	if err := readKeySSHField(source, outer[:]); err != nil {
		return SigningKey{}, err
	}
	outerSource := bytes.NewReader(outer[:])
	outerPublic, err := readKeyPublic(outerSource)
	if err != nil {
		return SigningKey{}, err
	}
	extent, err := readKeyUint32(source)
	if err != nil || extent%8 != 0 {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err)
	}
	privateSource := &io.LimitedReader{R: source, N: int64(extent)}
	first, err := readKeyUint32(privateSource)
	if err != nil {
		return SigningKey{}, err
	}
	second, err := readKeyUint32(privateSource)
	if err != nil || first != second {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err)
	}
	public, err := readKeyPublic(privateSource)
	if err != nil || public != outerPublic {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err)
	}
	var private [openSSHPrivateBytes]byte
	defer clear(private[:])
	if err := readKeySSHField(privateSource, private[:]); err != nil {
		return SigningKey{}, err
	}
	comment, err := readKeyUint32(privateSource)
	if err != nil {
		return SigningKey{}, err
	}
	if _, err := io.CopyN(io.Discard, privateSource, int64(comment)); err != nil {
		return SigningKey{}, err
	}
	if privateSource.N > 7 {
		return SigningKey{}, core.ErrKeygenContract
	}
	for padding := byte(1); privateSource.N > 0; padding++ {
		var value [1]byte
		if _, err := io.ReadFull(privateSource, value[:]); err != nil {
			return SigningKey{}, err
		}
		if value[0] != padding {
			return SigningKey{}, core.ErrKeygenContract
		}
	}
	key, err := AdoptPrivateKey(private[:])
	if err != nil {
		return SigningKey{}, err
	}
	derived, err := key.PublicKey()
	if err != nil || derived != public {
		return SigningKey{}, errors.Join(core.ErrKeygenContract, err, key.Destroy())
	}
	return key, nil
}

func readKeyPublic(source io.Reader) (core.Ed25519PublicKey, error) {
	var algorithm [len(openSSHAlgorithm)]byte
	if err := readKeySSHField(source, algorithm[:]); err != nil {
		return core.Ed25519PublicKey{}, err
	}
	if string(algorithm[:]) != openSSHAlgorithm {
		return core.Ed25519PublicKey{}, core.ErrKeygenContract
	}
	var public [openSSHPublicBytes]byte
	if err := readKeySSHField(source, public[:]); err != nil {
		return core.Ed25519PublicKey{}, err
	}
	return core.NewEd25519PublicKey(public[:])
}

func readKeyUint32(source io.Reader) (uint32, error) {
	var value [4]byte
	_, err := io.ReadFull(source, value[:])
	return binary.BigEndian.Uint32(value[:]), err
}

func readKeySSHField(source io.Reader, destination []byte) error {
	length, err := readKeyUint32(source)
	if err != nil || uint64(length) != uint64(len(destination)) {
		return errors.Join(core.ErrKeygenContract, err)
	}
	_, err = io.ReadFull(source, destination)
	return err
}

// WriteOpenSSH streams unencrypted OpenSSH framing and PEM through
// native base64 encoding. The caller owns the destination and any partial
// output on refusal. Check integers come from Go's production entropy source.
func WriteOpenSSH(ctx context.Context, request OpenSSHWriteRequest) error {
	if ctx == nil {
		return errors.Join(core.ErrKeygenContract, core.ErrNilContext)
	}
	if err := errors.Join(ctx.Err(), context.Cause(ctx), request.Validate()); err != nil {
		return contractError(err)
	}
	private, err := request.Key.PrivateKey()
	if err != nil {
		return err
	}
	defer clear(private)
	public := private[SeedSize:]
	check, err := RandomUint64()
	if err != nil {
		return err
	}
	stream := keyStream{ctx: ctx, destination: request.Destination}
	if _, err := io.WriteString(stream, "-----BEGIN "+request.Label.String()+"-----\n"); err != nil {
		return contractError(err)
	}
	lines := &keyPEMLines{destination: stream}
	encoded := base64.NewEncoder(base64.StdEncoding, lines)
	if err := writeOpenSSHBinary(encoded, request.Comment, private, public, uint32(check&math.MaxUint32)); err != nil {
		return contractError(errors.Join(err, encoded.Close()))
	}
	if err := encoded.Close(); err != nil {
		return contractError(err)
	}
	if lines.column != 0 {
		if _, err := io.WriteString(stream, "\n"); err != nil {
			return contractError(err)
		}
	}
	_, err = io.WriteString(stream, "-----END "+request.Label.String()+"-----\n")
	return contractErrorIfPresent(errors.Join(err, ctx.Err(), context.Cause(ctx)))
}

func writeOpenSSHBinary(destination io.Writer, comment string, private, public []byte, check uint32) error {
	if _, err := io.WriteString(destination, openSSHMagic); err != nil {
		return err
	}
	for range 2 {
		if err := writeKeySSHField(destination, []byte(openSSHCipher)); err != nil {
			return err
		}
	}
	if err := writeKeyUint32(destination, 0); err != nil {
		return err
	}
	if err := writeKeyUint32(destination, 1); err != nil {
		return err
	}
	if err := writeKeyUint32(destination, uint32(openSSHPublicExtent)); err != nil {
		return err
	}
	if err := writeKeyPublic(destination, public); err != nil {
		return err
	}
	unpadded := uint32(openSSHPrivateFixed) + uint32(len(comment))
	padding := (8 - unpadded%8) % 8
	if err := writeKeyUint32(destination, unpadded+padding); err != nil {
		return err
	}
	for range 2 {
		if err := writeKeyUint32(destination, check); err != nil {
			return err
		}
	}
	if err := writeKeyPublic(destination, public); err != nil {
		return err
	}
	if err := writeKeySSHField(destination, private); err != nil {
		return err
	}
	if err := writeKeyUint32(destination, uint32(len(comment))); err != nil {
		return err
	}
	if _, err := io.WriteString(destination, comment); err != nil {
		return err
	}
	for value := byte(1); uint32(value) <= padding; value++ {
		if _, err := destination.Write([]byte{value}); err != nil {
			return err
		}
	}
	return nil
}

func writeKeyPublic(destination io.Writer, public []byte) error {
	if err := writeKeySSHField(destination, []byte(openSSHAlgorithm)); err != nil {
		return err
	}
	return writeKeySSHField(destination, public)
}

func writeKeySSHField(destination io.Writer, value []byte) error {
	if err := writeKeyUint32(destination, uint32(len(value))); err != nil {
		return err
	}
	_, err := destination.Write(value)
	return err
}

func writeKeyUint32(destination io.Writer, value uint32) error {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], value)
	_, err := destination.Write(bytes[:])
	return err
}

type keyPEMLines struct {
	destination io.Writer
	column      int
}

func (s *keyPEMLines) Write(p []byte) (n int, resultErr error) {
	for n < len(p) {
		size := min(64-s.column, len(p)-n)
		written, err := s.destination.Write(p[n : n+size])
		n += written
		s.column += written
		if err != nil {
			return n, err
		}
		if written != size {
			return n, io.ErrShortWrite
		}
		if s.column == 64 {
			if _, err := io.WriteString(s.destination, "\n"); err != nil {
				return n, err
			}
			s.column = 0
		}
	}
	return n, nil
}

func contractErrorIfPresent(err error) error {
	if err != nil {
		return contractError(err)
	}
	return nil
}
