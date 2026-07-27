package flickr

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestClientPaginationPublicEnforcementAndOptionalEXIF(t *testing.T) {
	handler := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("oauth_signature") == "" {
			t.Error("request was not signed")
		}
		switch method := r.URL.Query().Get("method"); method {
		case "flickr.photosets.getList":
			if r.URL.Query().Get("page") == "1" {
				return jsonResponse(`{"stat":"ok","photosets":{"page":1,"pages":2,"photoset":[{"id":"a","title":{"_content":"A"},"description":{"_content":"first"}}]}}`), nil
			}
			return jsonResponse(`{"stat":"ok","photosets":{"page":2,"pages":2,"photoset":[{"id":"b","title":{"_content":"B"},"description":{"_content":"second"}}]}}`), nil
		case "flickr.photosets.getPhotos":
			if r.URL.Query().Get("privacy_filter") != "1" || r.URL.Query().Get("media") != "photos" || r.URL.Query().Get("user_id") != "user" {
				t.Error("missing public-photo constraints")
			}
			return jsonResponse(`{"stat":"ok","photoset":{"page":1,"pages":1,"photo":[{"id":"public","title":"Public","description":{"_content":"desc"},"datetaken":"2026-01-02 03:04:05","url_m":"https://img/m.jpg","url_l":"https://img/l.jpg","width_m":"640","height_m":"480","width_l":1600,"height_l":1200,"latitude":"40.1","longitude":"-8.2"},{"id":"private","title":"Private"}]}}`), nil
		case "flickr.photos.getInfo":
			id := r.URL.Query().Get("photo_id")
			public := 1
			if id == "private" {
				public = 0
			}
			return jsonResponse(`{"stat":"ok","photo":{"visibility":{"ispublic":` + strconv.Itoa(public) + `},"urls":{"url":[{"_content":"https://flickr.test/` + id + `"}]}}}`), nil
		case "flickr.photos.getExif":
			return jsonResponse(`{"stat":"fail","code":2,"message":"Permission denied"}`), nil
		default:
			t.Errorf("unexpected method %q", method)
			return jsonResponse(`{"stat":"fail","code":1,"message":"unexpected"}`), nil
		}
	})
	client := NewClient(Config{APIKey: "key", APISecret: "secret", PublicBaseURL: "https://portfolio.test", RESTURL: "https://api.test/rest"})
	client.HTTP.Transport = handler
	albums, err := client.Albums(t.Context(), Token{Value: "token", Secret: "token-secret"}, "user")
	if err != nil || len(albums) != 2 || albums[1].Position != 1 {
		t.Fatalf("albums = %+v, %v", albums, err)
	}
	photos, err := client.AlbumPhotos(t.Context(), Token{Value: "token", Secret: "token-secret"}, "user", albums[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 || photos[0].ID != "public" || photos[0].Width != 1600 || photos[0].Height != 1200 || photos[0].Location != "40.1, -8.2" {
		t.Fatalf("photos = %+v", photos)
	}
	if photos[0].Camera != "" {
		t.Fatal("hidden EXIF should remain blank")
	}
}

func TestClientHandlesStringPaginationFields(t *testing.T) {
	client := NewClient(Config{APIKey: "key", APISecret: "secret", PublicBaseURL: "https://portfolio.test", RESTURL: "https://api.test/rest"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Query().Get("method") {
		case "flickr.photosets.getList":
			return jsonResponse(`{"stat":"ok","photosets":{"page":"1","pages":"1","photoset":[{"id":"a","title":{"_content":"A"},"description":{"_content":"d"}}]}}`), nil
		case "flickr.photosets.getPhotos":
			return jsonResponse(`{"stat":"ok","photoset":{"page":"1","pages":"1","photo":[{"id":"p","title":"P","latitude":40.1,"longitude":-8.2}]}}`), nil
		case "flickr.photos.getInfo":
			return jsonResponse(`{"stat":"ok","photo":{"visibility":{"ispublic":1},"urls":{"url":[{"_content":"https://flickr.test/p"}]}}}`), nil
		case "flickr.photos.getExif":
			return jsonResponse(`{"stat":"fail","code":2,"message":"Permission denied"}`), nil
		default:
			t.Errorf("unexpected method %q", r.URL.Query().Get("method"))
			return jsonResponse(`{"stat":"fail","code":1,"message":"unexpected"}`), nil
		}
	})
	albums, err := client.Albums(t.Context(), Token{Value: "token", Secret: "token-secret"}, "user")
	if err != nil || len(albums) != 1 {
		t.Fatalf("albums = %+v, %v", albums, err)
	}
	photos, err := client.AlbumPhotos(t.Context(), Token{Value: "token", Secret: "token-secret"}, "user", albums[0])
	if err != nil || len(photos) != 1 {
		t.Fatalf("photos = %+v, %v", photos, err)
	}
}

func TestClientAPIErrorsAndTimeout(t *testing.T) {
	client := NewClient(Config{APIKey: "key", APISecret: "secret", PublicBaseURL: "https://portfolio.test", RESTURL: "https://api.test/rest"})
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"stat":"fail","code":100,"message":"Invalid API Key"}`), nil
	})
	if _, err := client.Albums(t.Context(), Token{}, "u"); err == nil {
		t.Fatal("API error accepted")
	} else if _, ok := err.(*APIError); !ok {
		t.Fatalf("error type = %T", err)
	}
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		select {
		case <-time.After(50 * time.Millisecond):
			return jsonResponse(`{"stat":"ok","photosets":{"page":1,"pages":1}}`), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	client.HTTP.Timeout = 5 * time.Millisecond
	if _, err := client.Albums(t.Context(), Token{}, "u"); err == nil {
		t.Fatal("timeout accepted")
	}
}

func TestOAuthFlowUsesSignedGETAndReadOnlyPermission(t *testing.T) {
	client := NewClient(Config{APIKey: "key", APISecret: "secret", PublicBaseURL: "https://portfolio.test", RequestTokenURL: "https://oauth.test/request", AuthorizeURL: "https://oauth.test/authorize", AccessTokenURL: "https://oauth.test/access"})
	client.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Query().Get("oauth_signature") == "" {
			t.Errorf("OAuth request = %s %s", r.Method, r.URL.String())
		}
		if r.URL.Path == "/request" {
			if r.URL.Query().Get("oauth_callback") != "https://portfolio.test/admin/flickr/callback" {
				t.Error("callback missing")
			}
			return jsonResponse("oauth_callback_confirmed=true&oauth_token=request&oauth_token_secret=request-secret"), nil
		}
		if r.URL.Query().Get("oauth_verifier") != "verifier" {
			t.Error("verifier missing")
		}
		return jsonResponse("oauth_token=access&oauth_token_secret=access-secret&user_nsid=user%40N01&username=rafael"), nil
	})
	request, authorize, err := client.RequestToken(t.Context(), "https://portfolio.test/admin/flickr/callback")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(authorize, "perms=read") {
		t.Fatalf("authorize URL = %s", authorize)
	}
	access, user, username, err := client.AccessToken(t.Context(), request, "verifier")
	if err != nil || access.Value != "access" || user != "user@N01" || username != "rafael" {
		t.Fatalf("access = %+v %q %q %v", access, user, username, err)
	}
}
