package paygateway

import (
	"net/url"
	"testing"
)

func TestCanonicalSignStringSortsKeysAndValues(t *testing.T) {
	values := url.Values{}
	values.Add("timestamp", "2")
	values.Add("app_id", "snsgo")
	values.Add("nonce", "b")
	values.Add("nonce", "a")
	values.Add("sign", "ignored")

	got := CanonicalSignString(values)
	want := "app_id=snsgo&nonce=a&nonce=b&timestamp=2"
	if got != want {
		t.Fatalf("CanonicalSignString() = %q, want %q", got, want)
	}
}

func TestSignReturnsHMACSHA256Hex(t *testing.T) {
	values := url.Values{}
	values.Set("app_id", "snsgo")
	values.Set("nonce", "n_001")
	values.Set("request_id", "req_001")
	values.Set("timestamp", "1782921600")

	got := Sign("secret", values)
	want := "907a020edfc00d678c9a05f8703797f5e35b425f2809f3b840d602e93075aa04"
	if got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}
}
