package idp

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type ClaimSet struct {
	Iss   string `json:"iss"`             // email address of the client_id of the application making the access token request
	Scope string `json:"scope,omitempty"` // space-delimited list of the permissions the application requests
	Aud   string `json:"aud"`             // descriptor of the intended target of the assertion (Optional).
	Exp   int64  `json:"exp"`             // the expiration time of the assertion (seconds since Unix epoch)
	Iat   int64  `json:"iat"`             // the time the assertion was issued (seconds since Unix epoch)
	Typ   string `json:"typ,omitempty"`   // token type (Optional).

	// Email for which the application is requesting delegated access (Optional).
	Sub string `json:"sub,omitempty"`

	// The old name of Sub. Client keeps setting Prn to be
	// complaint with legacy OAuth 2.0 providers. (Optional)
	Prn string `json:"prn,omitempty"`

	// See http://tools.ietf.org/html/draft-jones-json-web-token-10#section-4.3
	// This array is marshalled using custom code (see (c *ClaimSet) encode()).
	PrivateClaims map[string]interface{} `json:"-"`
}

func (c *ClaimSet) encode() (string, error) {
	// Reverting time back for machines whose time is not perfectly in sync.
	// If client machine's time is in the future according
	// to Google servers, an access token will not be issued.
	now := time.Now().Add(-10 * time.Second)
	if c.Iat == 0 {
		c.Iat = now.Unix()
	}
	if c.Exp == 0 {
		c.Exp = now.Add(time.Hour).Unix()
	}
	if c.Exp < c.Iat {
		return "", fmt.Errorf("jws: invalid Exp = %v; must be later than Iat = %v", c.Exp, c.Iat)
	}

	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	if len(c.PrivateClaims) == 0 {
		return base64.RawURLEncoding.EncodeToString(b), nil
	}

	// Marshal private claim set and then append it to b.
	prv, err := json.Marshal(c.PrivateClaims)
	if err != nil {
		return "", fmt.Errorf("jws: invalid map of private claims %v", c.PrivateClaims)
	}

	// Concatenate public and private claim JSON objects.
	if !bytes.HasSuffix(b, []byte{'}'}) {
		return "", fmt.Errorf("jws: invalid JSON %s", b)
	}
	if !bytes.HasPrefix(prv, []byte{'{'}) {
		return "", fmt.Errorf("jws: invalid JSON %s", prv)
	}
	b[len(b)-1] = ','         // Replace closing curly brace with a comma.
	b = append(b, prv[1:]...) // Append private claims.
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type Header struct {
	// The algorithm used for signature.
	Algorithm string `json:"alg"`

	// Represents the token type.
	Typ string `json:"typ"`

	// The optional hint of which key is being used.
	KeyID string `json:"kid,omitempty"`
}

func (h *Header) encode() (string, error) {
	b, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Signer returns a signature for the given data.
type Signer func(data []byte) (sig []byte, err error)

// EncodeWithSigner encodes a header and claim set with the provided signer.
func EncodeWithSigner(header *Header, c *ClaimSet, sg Signer) (string, error) {
	head, err := header.encode()
	if err != nil {
		return "", err
	}
	cs, err := c.encode()
	if err != nil {
		return "", err
	}
	ss := fmt.Sprintf("%s.%s", head, cs)
	sig, err := sg([]byte(ss))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", ss, base64.RawURLEncoding.EncodeToString(sig)), nil
}

// Encode encodes a signed JWS with provided header and claim set.
// This invokes EncodeWithSigner using crypto/rsa.SignPKCS1v15 with the given RSA private key.
func Encode(header *Header, c *ClaimSet, key *rsa.PrivateKey) (string, error) {
	sg := func(data []byte) (sig []byte, err error) {
		h := sha256.New()
		h.Write(data)
		return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h.Sum(nil))
	}
	return EncodeWithSigner(header, c, sg)
}
