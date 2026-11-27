package cloudflare

import (
	"encoding/xml"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func decodeR2ListPage(source io.Reader, request R2ListRequest) (R2ListPage, error) {
	decoder := xml.NewDecoder(source)
	root, err := r2ListRoot(decoder)
	if err != nil {
		return R2ListPage{}, err
	}
	var page R2ListPage
	names := [...]string{"Name", "Prefix", "EncodingType", "IsTruncated", "KeyCount", "MaxKeys", "ContinuationToken", "NextContinuationToken"}
	fields := [len(names)]*string{}
	err = r2ListChildren(decoder, root.Name, func(child xml.StartElement) error {
		if child.Name.Local != "Contents" {
			return r2ListScalar(decoder, child.Name, names[:], fields[:])
		}
		if len(page.Objects) >= int(request.MaximumKeys) {
			return core.ErrCloudflareResponse
		}
		object, err := r2ListObject(decoder, child.Name, request.Prefix)
		if err != nil {
			return err
		}
		page.Objects = append(page.Objects, object)
		return nil
	})
	if err != nil {
		return R2ListPage{}, err
	}
	if _, err := r2ListToken(decoder, false); err != io.EOF {
		return R2ListPage{}, responseError(err)
	}
	page.Continuation, err = r2ListCursor(request, len(page.Objects), fields)
	if err != nil {
		return R2ListPage{}, err
	}
	return page, nil
}

func r2ListRoot(decoder *xml.Decoder) (xml.StartElement, error) {
	token, err := r2ListToken(decoder, true)
	if err != nil {
		return xml.StartElement{}, responseError(err)
	}
	root, ok := token.(xml.StartElement)
	if !ok || root.Name.Local != "ListBucketResult" || !r2XMLNamespace(root.Name.Space) || !r2XMLRootAttributes(root.Attr) {
		return xml.StartElement{}, core.ErrCloudflareResponse
	}
	return root, nil
}

func r2ListToken(decoder *xml.Decoder, allowDeclaration bool) (xml.Token, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.Comment:
		case xml.ProcInst:
			if !allowDeclaration || value.Target != "xml" {
				return nil, core.ErrCloudflareResponse
			}
			allowDeclaration = false
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return nil, core.ErrCloudflareResponse
			}
		default:
			return token, nil
		}
	}
}

func r2ListChildren(decoder *xml.Decoder, parent xml.Name, visit func(xml.StartElement) error) error {
	for {
		token, err := r2ListToken(decoder, false)
		if err != nil {
			return responseError(err)
		}
		switch child := token.(type) {
		case xml.StartElement:
			if child.Name.Space != parent.Space || len(child.Attr) != 0 {
				return core.ErrCloudflareResponse
			}
			if err := visit(child); err != nil {
				return err
			}
		case xml.EndElement:
			if child.Name == parent {
				return nil
			}
			return core.ErrCloudflareResponse
		default:
			return core.ErrCloudflareResponse
		}
	}
}

func r2ListScalar(decoder *xml.Decoder, name xml.Name, names []string, fields []*string) error {
	index := slices.Index(names, name.Local)
	if index < 0 || fields[index] != nil {
		return core.ErrCloudflareResponse
	}
	value, err := r2XMLText(decoder, name)
	if err != nil {
		return err
	}
	fields[index] = &value
	return nil
}

func r2ListValue(field *string) string {
	if field == nil {
		return ""
	}
	return *field
}

func r2ListCursor(request R2ListRequest, count int, fields [8]*string) (R2ListContinuation, error) {
	prefix, err := url.PathUnescape(r2ListValue(fields[1]))
	if err != nil || prefix != request.Prefix.value || r2ListValue(fields[0]) != request.Bucket.value ||
		r2ListValue(fields[2]) != "url" || r2ListValue(fields[4]) != strconv.Itoa(count) ||
		r2ListValue(fields[5]) != strconv.Itoa(int(request.MaximumKeys)) {
		return R2ListContinuation{}, core.ErrCloudflareResponse
	}
	if fields[6] != nil && *fields[6] != request.Continuation.value {
		return R2ListContinuation{}, core.ErrCloudflareResponse
	}
	switch r2ListValue(fields[3]) {
	case "false":
		if fields[7] != nil {
			return R2ListContinuation{}, core.ErrCloudflareResponse
		}
		return R2ListContinuation{}, nil
	case "true":
		next, err := ParseR2ListContinuation(r2ListValue(fields[7]))
		if err != nil || next == request.Continuation {
			return R2ListContinuation{}, responseError(err)
		}
		return next, nil
	default:
		return R2ListContinuation{}, core.ErrCloudflareResponse
	}
}

func r2ListObject(decoder *xml.Decoder, parent xml.Name, prefix R2ListPrefix) (R2ListedObject, error) {
	names := [...]string{"Key", "ETag", "Size", "LastModified", "StorageClass"}
	fields := [len(names)]*string{}
	if err := r2ListChildren(decoder, parent, func(child xml.StartElement) error {
		return r2ListScalar(decoder, child.Name, names[:], fields[:])
	}); err != nil {
		return R2ListedObject{}, err
	}
	keyText, err := url.PathUnescape(r2ListValue(fields[0]))
	if err != nil || !strings.HasPrefix(keyText, prefix.value) {
		return R2ListedObject{}, core.ErrCloudflareResponse
	}
	key, err := ParseR2Key(keyText)
	if err != nil {
		return R2ListedObject{}, responseError(err)
	}
	etag := r2ListValue(fields[1])
	extent, err := strconv.ParseUint(r2ListValue(fields[2]), 10, 64)
	if err != nil || extent > core.CloudflareR2MultipartMaximumObjectBytes || len(etag) > r2ListETagMaximumBytes || !r2ETag(etag) {
		return R2ListedObject{}, core.ErrCloudflareResponse
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return R2ListedObject{}, responseError(err)
	}
	modified, err := temporal.ParseRFC3339(r2ListValue(fields[3]))
	if err != nil {
		return R2ListedObject{}, responseError(err)
	}
	return R2ListedObject{Key: key, ETag: etag, Bytes: length, LastModified: modified}, nil
}
