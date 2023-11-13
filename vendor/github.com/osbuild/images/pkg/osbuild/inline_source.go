package osbuild

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type InlineSource struct {
	Items map[string]InlineSourceItem `json:"items"`
}

func (InlineSource) isSource() {}

type InlineSourceItem struct {
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

func NewInlineSource() *InlineSource {
	return &InlineSource{
		Items: make(map[string]InlineSourceItem),
	}
}

// AddItem a new item to the source. Well hash and encode that data
// and return the checksum.
func (s *InlineSource) AddItem(data string) string {

	dataBytes := []byte(data)

	encoded := base64.StdEncoding.EncodeToString(dataBytes)
	name := fmt.Sprintf("sha256:%x", sha256.Sum256(dataBytes))

	s.Items[name] = InlineSourceItem{
		Encoding: "base64",
		Data:     encoded,
	}

	return name
}
