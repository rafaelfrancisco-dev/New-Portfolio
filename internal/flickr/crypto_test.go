package flickr

import (
	"net/http"
	"net/url"
	"testing"
)

func TestOAuthSignatureRFCExample(t *testing.T) {
	params := url.Values{
		"file": {"vacation.jpg"}, "size": {"original"},
		"oauth_consumer_key": {"dpf43f3p2l4k3l03"}, "oauth_token": {"nnch734d00sl2jdk"},
		"oauth_nonce": {"kllo9940pd9333jh"}, "oauth_timestamp": {"1191242096"},
		"oauth_signature_method": {"HMAC-SHA1"}, "oauth_version": {"1.0"},
	}
	got := Sign(http.MethodGet, "http://photos.example.net/photos", params, "kd94hf93k423kf44", "pfkkdhi9sl3r4s00")
	if got != "tR3+Ty81lMeYAr/Fid0kMTYa/WM=" {
		t.Fatalf("signature = %q", got)
	}
}

func TestTokenEncryptionAndWrongKey(t *testing.T) {
	cipher, err := NewCipher("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("secret access token")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := cipher.Decrypt(encrypted)
	if err != nil || plain != "secret access token" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
	wrong, _ := NewCipher("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=")
	if _, err := wrong.Decrypt(encrypted); err == nil {
		t.Fatal("wrong key unexpectedly decrypted token")
	}
	if _, err := NewCipher("short"); err == nil {
		t.Fatal("invalid key accepted")
	}
}
