package cmd

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type apnsRequest struct {
	URL     string
	Headers http.Header
	Body    []byte
}

func apnsRequests(cfg providerWorkerConfig, req workerDeliveryRequest, bearer string) ([]apnsRequest, error) {
	host := "https://api.sandbox.push.apple.com"
	if strings.TrimSpace(strings.ToLower(cfg.APNSEnvironment)) == "production" {
		host = "https://api.push.apple.com"
	}
	requests := []apnsRequest{}
	for _, target := range req.Devices {
		token := apnsDeviceToken(req.Operation, target)
		if token == "" {
			continue
		}
		body, err := apnsPayload(req)
		if err != nil {
			return nil, err
		}
		headers := http.Header{}
		headers.Set("authorization", "bearer "+bearer)
		headers.Set("apns-topic", cfg.APNSTopic)
		headers.Set("apns-push-type", apnsPushType(req.Operation))
		headers.Set("content-type", "application/json")
		requests = append(requests, apnsRequest{URL: host + "/3/device/" + token, Headers: headers, Body: body})
	}
	return requests, nil
}

func apnsBearerToken(cfg providerWorkerConfig, now time.Time) (string, error) {
	key, err := parseAPNSPrivateKey(cfg.APNSPrivateKeyPEM)
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": "ES256", "kid": strings.TrimSpace(cfg.APNSKeyID)})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{"iss": strings.TrimSpace(cfg.APNSTeamID), "iat": now.Unix()})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	signature := append(fixedWidth(r, 32), fixedWidth(s, 32)...)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func parseAPNSPrivateKey(pemData string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, fmt.Errorf("invalid APNs private key PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if ecKey, ok := key.(*ecdsa.PrivateKey); ok {
			return ecKey, nil
		}
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid APNs private key")
	}
	return key, nil
}

func fixedWidth(value *big.Int, width int) []byte {
	out := make([]byte, width)
	bytes := value.Bytes()
	copy(out[width-len(bytes):], bytes)
	return out
}

func apnsDeviceToken(operation string, target workerTargetDevice) string {
	if operation == "activity_start" && strings.TrimSpace(target.PushToStartToken) != "" {
		return strings.TrimSpace(target.PushToStartToken)
	}
	return strings.TrimSpace(target.PushToken)
}

func apnsPushType(operation string) string {
	if strings.HasPrefix(operation, "activity_") {
		return "liveactivity"
	}
	return "alert"
}

func apnsPayload(req workerDeliveryRequest) ([]byte, error) {
	payload := map[string]any{"aps": map[string]any{}}
	switch {
	case req.Operation == "notification":
		alert := map[string]string{}
		if req.Notification != nil {
			alert["title"] = req.Notification["title"]
			alert["body"] = req.Notification["body"]
		}
		payload["aps"].(map[string]any)["alert"] = alert
	case strings.HasPrefix(req.Operation, "activity_"):
		payload["aps"].(map[string]any)["event"] = strings.TrimPrefix(req.Operation, "activity_")
		if req.Activity != nil {
			payload["content-state"] = req.Activity.State
			payload["beam-sequence"] = req.Activity.Sequence
		}
	default:
		return nil, fmt.Errorf("unsupported APNs operation %q", req.Operation)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(body), nil
}
