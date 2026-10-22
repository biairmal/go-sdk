package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
)

// jwksCacheControl lets verifiers cache the key set for five minutes, so a key
// published as "next" reaches every verifier before it starts signing.
const jwksCacheControl = "public, max-age=300"

// PublicKey is one RSA verification key published in a JWKS, identified by its kid.
type PublicKey struct {
	KID string
	Key *rsa.PublicKey
}

// JWKSHandler serves the RFC 7517 key set returned by keys. keys is called on
// every request, so a rotating key set is reflected without rebuilding the
// handler. Entries with a nil key are skipped.
func JWKSHandler(keys func() []PublicKey) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var current []PublicKey
		if keys != nil {
			current = keys()
		}
		set := jwkSet{Keys: make([]jwk, 0, len(current))}
		for _, k := range current {
			if k.Key == nil {
				continue
			}
			set.Keys = append(set.Keys, rsaToJWK(k))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", jwksCacheControl)
		if err := json.NewEncoder(w).Encode(set); err != nil {
			// Header already sent; cannot return error to client.
			return
		}
	}
}

// rsaToJWK converts a public key into its JWK form (base64url n/e).
func rsaToJWK(k PublicKey) jwk {
	return jwk{
		Kty: "RSA",
		Kid: k.KID,
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(k.Key.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.Key.E)).Bytes()),
	}
}
