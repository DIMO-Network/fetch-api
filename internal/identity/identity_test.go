package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const expectedVehicleDID = "did:erc721:137:0xbA5738a18d83D41847dfFbDC6101d37C69c9B0cF:186612"

// Success case 1: aftermarketDevice has vehicle, syntheticDevice is null (with error in path).
func TestGetLinkedDIDForDevice_SuccessAftermarket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
  "errors": [
    {
      "message": "invalid contract address '0x9c94C395cBcBDe662235E0A9d3bB87Ad708561BA' in token did",
      "path": ["syntheticDevice"]
    }
  ],
  "data": {
    "aftermarketDevice": {
      "vehicle": {
        "tokenDID": "` + expectedVehicleDID + `"
      }
    },
    "syntheticDevice": null
  }
}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.GetLinkedDIDForDevice(context.Background(), "did:test:device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectedVehicleDID {
		t.Errorf("got %q, want %q", got, expectedVehicleDID)
	}
}

// Success case 2: syntheticDevice has vehicle, aftermarketDevice is null (with error in path).
func TestGetLinkedDIDForDevice_SuccessSynthetic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
  "errors": [
    {
      "message": "invalid contract address '0x9c94C395cBcBDe662235E0A9d3bB87Ad708561BA' in token did",
      "path": ["aftermarketDevice"]
    }
  ],
  "data": {
    "syntheticDevice": {
      "vehicle": {
        "tokenDID": "` + expectedVehicleDID + `"
      }
    },
    "aftermarketDevice": null
  }
}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.GetLinkedDIDForDevice(context.Background(), "did:test:device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectedVehicleDID {
		t.Errorf("got %q, want %q", got, expectedVehicleDID)
	}
}

// Failure: both null.
func TestGetLinkedDIDForDevice_BothNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
  "errors": [{"message": "not found"}],
  "data": {
    "aftermarketDevice": null,
    "syntheticDevice": null
  }
}`))
	}))
	defer server.Close()

	client := New(server.URL)
	_, err := client.GetLinkedDIDForDevice(context.Background(), "did:test:device")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errDeviceNotFound) {
		t.Errorf("expected errDeviceNotFound, got %v", err)
	}
}
