package paygateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

func Sign(secret string, values url.Values) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(CanonicalSignString(values)))
	return hex.EncodeToString(mac.Sum(nil))
}

func CanonicalSignString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "sign" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, value := range items {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "&")
}
