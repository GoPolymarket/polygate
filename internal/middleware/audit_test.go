package middleware

import (
	"encoding/json"
	"testing"
)

func TestRedactAuditBodyOrders(t *testing.T) {
	body := []byte(`{"token_id":"1","signature":"0xdead","signer":"0xbeef","l2":{"api_key":"k","api_secret":"s","api_passphrase":"p"}}`)
	out := redactAuditBody("/v1/orders", body)

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if data["signature"] == "0xdead" {
		t.Fatalf("signature not redacted")
	}
	if data["signer"] == "0xbeef" {
		t.Fatalf("signer not redacted")
	}
	if l2, ok := data["l2"].(map[string]interface{}); ok {
		if l2["api_key"] == "k" || l2["api_secret"] == "s" || l2["api_passphrase"] == "p" {
			t.Fatalf("l2 creds not redacted")
		}
	}
}

func TestRedactAuditBodyNonSensitivePath(t *testing.T) {
	body := []byte(`{"ok":true}`)
	out := redactAuditBody("/health", body)
	if out != string(body) {
		t.Fatalf("unexpected redaction on non-sensitive path")
	}
}

func TestRedactAuditBodyInvalidJSON(t *testing.T) {
	body := []byte("not-json")
	out := redactAuditBody("/v1/orders", body)
	if out != "[redacted]" {
		t.Fatalf("expected redacted placeholder for invalid json")
	}
}
