package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	algorithm       = "AWS4-HMAC-SHA256"
	terminationStr  = "aws4_request"
	timeFormat      = "20060102T150405Z"
	shortTimeFormat = "20060102"
)

// signRequest signs an HTTP request using AWS Signature Version 4.
func signRequest(req *http.Request, creds *AWSCredentials, service string) error {
	now := time.Now().UTC()
	amzDate := now.Format(timeFormat)
	dateStamp := now.Format(shortTimeFormat)

	// Set required headers
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)

	// Add session token if present (for temporary credentials)
	if creds.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", creds.SessionToken)
	}

	// Step 1: Create canonical request
	canonicalRequest, signedHeaders := createCanonicalRequest(req)

	// Step 2: Create string to sign
	credentialScope := fmt.Sprintf("%s/%s/%s/%s", dateStamp, creds.Region, service, terminationStr)
	stringToSign := createStringToSign(amzDate, credentialScope, canonicalRequest)

	// Step 3: Calculate signature
	signingKey := deriveSigningKey(creds.SecretAccessKey, dateStamp, creds.Region, service)
	signature := hmacSHA256Hex(signingKey, stringToSign)

	// Step 4: Add Authorization header
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		creds.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)

	return nil
}

// createCanonicalRequest creates the canonical request string for signing.
func createCanonicalRequest(req *http.Request) (string, string) {
	// Canonical URI
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// Canonical query string (sorted)
	canonicalQueryString := createCanonicalQueryString(req.URL.Query())

	// Canonical headers (sorted, lowercase)
	canonicalHeaders, signedHeaders := createCanonicalHeaders(req.Header, req.Host)

	// Payload hash
	payloadHash := hashPayload(req)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	return canonicalRequest, signedHeaders
}

// createCanonicalQueryString creates a sorted query string.
func createCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		for _, v := range values[k] {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(pairs, "&")
}

// createCanonicalHeaders creates canonical headers and signed headers list.
func createCanonicalHeaders(headers http.Header, host string) (string, string) {
	// Headers to sign (lowercase)
	signedHeadersList := []string{"host"}
	for k := range headers {
		lower := strings.ToLower(k)
		if lower == "host" || strings.HasPrefix(lower, "x-amz-") {
			if lower != "host" {
				signedHeadersList = append(signedHeadersList, lower)
			}
		}
	}
	sort.Strings(signedHeadersList)

	// Build canonical headers
	var canonicalHeaders strings.Builder
	for _, key := range signedHeadersList {
		var value string
		if key == "host" {
			value = host
		} else {
			// Find original header (case-insensitive)
			for k, v := range headers {
				if strings.ToLower(k) == key {
					value = strings.TrimSpace(v[0])
					break
				}
			}
		}
		canonicalHeaders.WriteString(key + ":" + value + "\n")
	}

	return canonicalHeaders.String(), strings.Join(signedHeadersList, ";")
}

// hashPayload returns the SHA256 hash of the request body.
func hashPayload(req *http.Request) string {
	if req.Body == nil {
		return sha256Hex([]byte(""))
	}

	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	return sha256Hex(body)
}

// createStringToSign creates the string to sign.
func createStringToSign(amzDate, credentialScope, canonicalRequest string) string {
	return strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
}

// deriveSigningKey derives the signing key using the secret key.
func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, terminationStr)
	return kSigning
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// hmacSHA256Hex computes HMAC-SHA256 and returns hex string.
func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

// sha256Hex computes SHA256 hash and returns hex string.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
