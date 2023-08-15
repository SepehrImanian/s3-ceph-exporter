package collector

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Collector struct {
	cephAccessKey  string
	cephSecretKey  string
	cephGatewayURL string
}

func generateSignature(method, path, secretKey string) string {
	stringToSign := fmt.Sprintf("%s\n\n\n%s\n%s", method, time.Now().UTC().Format(time.RFC1123), path)
	hmac := hmac.New(sha1.New, []byte(secretKey))
	hmac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(hmac.Sum(nil))
}

func createRequest(url, accessKey, signature string) (*http.Request, error) {
	// Generate the timestamp and date in UTC
	date := time.Now().UTC().Format(time.RFC1123)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Host", strings.Split(req.URL.Host, ":")[0])
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", accessKey, signature))
	return req, nil
}

func decodeResponse(responseBody io.Reader, target interface{}) error {
	return json.NewDecoder(responseBody).Decode(target)
}
