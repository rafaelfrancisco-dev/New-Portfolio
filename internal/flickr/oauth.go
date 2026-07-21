package flickr

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

func oauthEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func nonce() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func Sign(method, endpoint string, params url.Values, consumerSecret, tokenSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		values := append([]string(nil), params[k]...)
		sort.Strings(values)
		for _, v := range values {
			pairs = append(pairs, oauthEscape(k)+"="+oauthEscape(v))
		}
	}
	base := strings.ToUpper(method) + "&" + oauthEscape(endpoint) + "&" + oauthEscape(strings.Join(pairs, "&"))
	key := oauthEscape(consumerSecret) + "&" + oauthEscape(tokenSecret)
	mac := hmac.New(sha1.New, []byte(key))
	_, _ = mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func oauthParams(consumerKey, token string) (url.Values, error) {
	n, err := nonce()
	if err != nil {
		return nil, err
	}
	p := url.Values{
		"oauth_consumer_key": {consumerKey}, "oauth_nonce": {n},
		"oauth_signature_method": {"HMAC-SHA1"},
		"oauth_timestamp":        {fmt.Sprint(time.Now().Unix())}, "oauth_version": {"1.0"},
	}
	if token != "" {
		p.Set("oauth_token", token)
	}
	return p, nil
}
