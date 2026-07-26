package mokahr

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// aesIV is the fixed initialization vector MokaHR obfuscates responses with.
// Every careers page publishes it in its server-rendered state as `aesIv`,
// and it was identical on every tenant and session checked, so the client
// treats it as a constant rather than bootstrapping it from HTML. A tenant
// that ever ships a different one shows up as a decode failure naming this
// constant, not as silently wrong data.
const aesIV = "de7c21ed8d6f50fe"

// envelope is MokaHR's obfuscated response wrapper. Necromancer is the AES
// key for this very response — the site's own frontend decrypts exactly this
// way — so the scheme hides the payload from casual inspection and from
// nothing else. An endpoint may also answer in the clear, in which case both
// fields are empty and the body passes through untouched.
type envelope struct {
	Data        string `json:"data"`
	Necromancer string `json:"necromancer"`
}

// Transport deobfuscates MokaHR response bodies so the generated client
// decodes the payload described by openapi.yaml. Requests are untouched:
// they are ordinary JSON and need no authentication, cookies, or Referer.
type Transport struct {
	Base http.RoundTripper
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	rsp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("mokahr: read response body: %w", err)
	}
	plain, err := deobfuscate(body)
	if err != nil {
		return nil, err
	}
	rsp.Body = io.NopCloser(bytes.NewReader(plain))
	rsp.ContentLength = int64(len(plain))
	rsp.Header.Del("Content-Length")
	return rsp, nil
}

// deobfuscate returns body's plaintext, or body itself when the response is
// already in the clear.
func deobfuscate(body []byte) ([]byte, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Necromancer == "" || env.Data == "" {
		return body, nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("mokahr: decode response payload: %w", err)
	}
	plain, err := decryptCBC(ciphertext, []byte(env.Necromancer), []byte(aesIV))
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// decryptCBC reverses the AES-128-CBC + PKCS#7 obfuscation.
func decryptCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("mokahr: response key is not a valid AES key: %w", err)
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("mokahr: response IV %q is %d bytes, want %d", iv, len(iv), block.BlockSize())
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("mokahr: response payload is %d bytes, not a whole number of %d-byte blocks",
			len(ciphertext), block.BlockSize())
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	return unpadPKCS7(plain, block.BlockSize())
}

func unpadPKCS7(b []byte, blockSize int) ([]byte, error) {
	pad := int(b[len(b)-1])
	if pad == 0 || pad > blockSize || pad > len(b) {
		return nil, fmt.Errorf("mokahr: response padding byte is %d, outside 1..%d", pad, min(blockSize, len(b)))
	}
	for _, c := range b[len(b)-pad:] {
		if int(c) != pad {
			return nil, fmt.Errorf("mokahr: response padding is inconsistent; wrong key or IV %q", aesIV)
		}
	}
	return b[:len(b)-pad], nil
}
