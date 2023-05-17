package structs

import (
	"main/lib/helpers"
	"strconv"
	"strings"
)

// HEADERS -------------------------------------------------

type HttpDigest struct {
	Algorithm string
	Value     string
}

func FileDigestFromHeader(head string) *HttpDigest {
	l := strings.Split(head, ":")
	return &HttpDigest{Algorithm: l[0], Value: l[1]}
}

type HttpRange struct {
	Start int64
	End   int64
	Size  int64
}

func FileRangeFromHeader(head string) *HttpRange {
	a := strings.Split(head, " ")
	b := strings.Split(a[1], "/")
	c := strings.Split(b[0], "-")
	return &HttpRange{
		Start: int64(helpers.Must[int](strconv.Atoi(c[0]))),
		End:   int64(helpers.Must[int](strconv.Atoi(c[1]))),
		Size:  int64(helpers.Must[int](strconv.Atoi(b[1]))),
	}
}

type HttpDisposition struct {
	Kind     string
	FileName string
}

func FileDispositionFromHeader(head string) *HttpDisposition {
	a := strings.Split(head, " ")
	b := strings.Split(a[1], "=")
	return &HttpDisposition{
		Kind:     a[0][:len(a[0])-1],
		FileName: b[1][1 : len(b[1])-1],
	}
}

type HttpContentType struct {
	Type  string
	Value string
}

func FileContentTypeFromHeader(head string) *HttpContentType {
	a := strings.Split(head, "/")
	return &HttpContentType{
		Type:  a[0],
		Value: a[1],
	}
}

type HttpFileInfo struct {
	Range       *HttpRange
	Digest      *HttpDigest
	Disposition *HttpDisposition
	ContentType *HttpContentType
}

func (h *HttpFileInfo) ToSpec() *HttpFileSpec {
	return &HttpFileSpec{
		Size:          h.Range.Size,
		HashValue:     h.Digest.Value,
		HashAlgorithm: h.Digest.Algorithm,
		FileType:      h.ContentType.Value,
	}
}

type HttpFileSpec struct {
	Size          int64  `json:"size" groups:"local"`
	HashAlgorithm string `json:"hash_algorithm" groups:"local"`
	HashValue     string `json:"hash_value" groups:"local"`
	FileType      string `json:"file_type" groups:"local"`
}
