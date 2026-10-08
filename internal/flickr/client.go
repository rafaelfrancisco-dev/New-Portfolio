package flickr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIKey, APISecret, PublicBaseURL                       string
	RESTURL, RequestTokenURL, AuthorizeURL, AccessTokenURL string
}

func (c Config) Ready() bool { return c.APIKey != "" && c.APISecret != "" && c.PublicBaseURL != "" }

type Client struct {
	Config Config
	HTTP   *http.Client
}
type Token struct{ Value, Secret string }
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("Flickr API error %d: %s", e.Code, e.Message) }

func NewClient(config Config) *Client {
	if config.RESTURL == "" {
		config.RESTURL = "https://www.flickr.com/services/rest"
	}
	if config.RequestTokenURL == "" {
		config.RequestTokenURL = "https://www.flickr.com/services/oauth/request_token"
	}
	if config.AuthorizeURL == "" {
		config.AuthorizeURL = "https://www.flickr.com/services/oauth/authorize"
	}
	if config.AccessTokenURL == "" {
		config.AccessTokenURL = "https://www.flickr.com/services/oauth/access_token"
	}
	return &Client{Config: config, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) oauthRequest(ctx context.Context, endpoint string, params url.Values, token Token) (url.Values, error) {
	oauth, err := oauthParams(c.Config.APIKey, token.Value)
	if err != nil {
		return nil, err
	}
	for k, values := range params {
		for _, v := range values {
			oauth.Add(k, v)
		}
	}
	oauth.Set("oauth_signature", Sign(http.MethodGet, endpoint, oauth, c.Config.APISecret, token.Secret))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+oauth.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Flickr OAuth transport failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Flickr OAuth returned %s", resp.Status)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	if values.Get("oauth_problem") != "" {
		return nil, fmt.Errorf("Flickr OAuth: %s", values.Get("oauth_problem"))
	}
	return values, nil
}

func (c *Client) RequestToken(ctx context.Context, callback string) (Token, string, error) {
	v, err := c.oauthRequest(ctx, c.Config.RequestTokenURL, url.Values{"oauth_callback": {callback}}, Token{})
	if err != nil {
		return Token{}, "", err
	}
	if v.Get("oauth_callback_confirmed") != "true" {
		return Token{}, "", fmt.Errorf("Flickr did not confirm OAuth callback")
	}
	t := Token{v.Get("oauth_token"), v.Get("oauth_token_secret")}
	if t.Value == "" || t.Secret == "" {
		return Token{}, "", fmt.Errorf("Flickr returned an incomplete request token")
	}
	return t, c.Config.AuthorizeURL + "?perms=read&oauth_token=" + url.QueryEscape(t.Value), nil
}

func (c *Client) AccessToken(ctx context.Context, request Token, verifier string) (Token, string, string, error) {
	v, err := c.oauthRequest(ctx, c.Config.AccessTokenURL, url.Values{"oauth_verifier": {verifier}}, request)
	if err != nil {
		return Token{}, "", "", err
	}
	t := Token{v.Get("oauth_token"), v.Get("oauth_token_secret")}
	if t.Value == "" || t.Secret == "" || v.Get("user_nsid") == "" {
		return Token{}, "", "", fmt.Errorf("Flickr returned an incomplete access token")
	}
	return t, v.Get("user_nsid"), v.Get("username"), nil
}

type apiEnvelope struct {
	Stat    string `json:"stat"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) call(ctx context.Context, token Token, method string, params url.Values, target any) error {
	p, err := oauthParams(c.Config.APIKey, token.Value)
	if err != nil {
		return err
	}
	p.Set("method", method)
	p.Set("format", "json")
	p.Set("nojsoncallback", "1")
	for k, values := range params {
		for _, v := range values {
			p.Add(k, v)
		}
	}
	p.Set("oauth_signature", Sign(http.MethodGet, c.Config.RESTURL, p, c.Config.APISecret, token.Secret))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Config.RESTURL+"?"+p.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("call %s: Flickr returned %s", method, resp.Status)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	if envelope.Stat == "fail" {
		return &APIError{envelope.Code, envelope.Message}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	return nil
}

type Album struct {
	ID, Title, Description string
	Position               int
}
type Photo struct {
	ID, Title, Description, PageURL, SmallURL, LargeURL, TakenAt, Location string
	Width, Height                                                          int
	Camera, Lens, Exposure, Aperture, ISO, FocalLength                     string
}
type AlbumPhotos struct {
	Album  Album
	Photos []Photo
}

type contentValue struct {
	Content string `json:"_content"`
}
type albumJSON struct {
	ID          string       `json:"id"`
	Title       contentValue `json:"title"`
	Description contentValue `json:"description"`
}
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch v := value.(type) {
	case string:
		*s = flexString(v)
	case float64:
		*s = flexString(strconv.FormatFloat(v, 'f', -1, 64))
	case nil:
		*s = ""
	default:
		return fmt.Errorf("expected string or number")
	}
	return nil
}

type flexInt int

func (n *flexInt) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch v := value.(type) {
	case float64:
		*n = flexInt(int(v))
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("expected int or numeric string")
		}
		*n = flexInt(parsed)
	case nil:
		*n = 0
	default:
		return fmt.Errorf("expected int or numeric string")
	}
	return nil
}

type photoJSON struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Secret      string       `json:"secret"`
	Server      string       `json:"server"`
	Farm        int          `json:"farm"`
	Description contentValue `json:"description"`
	DateTaken   string       `json:"datetaken"`
	URLM        string       `json:"url_m"`
	URLL        string       `json:"url_l"`
	URLO        string       `json:"url_o"`
	WidthM      flexString   `json:"width_m"`
	HeightM     flexString   `json:"height_m"`
	WidthL      flexString   `json:"width_l"`
	HeightL     flexString   `json:"height_l"`
	WidthO      flexString   `json:"width_o"`
	HeightO     flexString   `json:"height_o"`
	Latitude    flexString   `json:"latitude"`
	Longitude   flexString   `json:"longitude"`
}

func (c *Client) Albums(ctx context.Context, token Token, userID string) ([]Album, error) {
	var out []Album
	for page := 1; ; page++ {
		var response struct {
			Photosets struct {
				Page, Pages flexInt
				Items       []albumJSON `json:"photoset"`
			} `json:"photosets"`
		}
		err := c.call(ctx, token, "flickr.photosets.getList", url.Values{"user_id": {userID}, "page": {strconv.Itoa(page)}, "per_page": {"500"}}, &response)
		if err != nil {
			return nil, err
		}
		for _, a := range response.Photosets.Items {
			out = append(out, Album{a.ID, a.Title.Content, a.Description.Content, len(out)})
		}
		if page >= int(response.Photosets.Pages) || response.Photosets.Pages == 0 {
			return out, nil
		}
	}
}

func parseInt(values ...string) int {
	for _, v := range values {
		if n, _ := strconv.Atoi(v); n > 0 {
			return n
		}
	}
	return 0
}
func (c *Client) AlbumPhotos(ctx context.Context, token Token, userID string, album Album) ([]Photo, error) {
	var out []Photo
	for page := 1; ; page++ {
		var response struct {
			Photoset struct {
				Page, Pages flexInt
				Owner       string
				Items       []photoJSON `json:"photo"`
			} `json:"photoset"`
		}
		params := url.Values{"photoset_id": {album.ID}, "user_id": {userID}, "privacy_filter": {"1"}, "media": {"photos"}, "page": {strconv.Itoa(page)}, "per_page": {"500"}, "extras": {"description,date_taken,geo,url_m,url_l,url_o,o_dims"}}
		if err := c.call(ctx, token, "flickr.photosets.getPhotos", params, &response); err != nil {
			return nil, err
		}
		for _, p := range response.Photoset.Items {
			photo, public, err := c.PhotoInfo(ctx, token, p.ID)
			if err != nil {
				return nil, err
			}
			if !public {
				continue
			}
			photo.Title = p.Title
			photo.Description = p.Description.Content
			photo.TakenAt = p.DateTaken
			photo.SmallURL = first(p.URLM, staticImageURL(p, "z"))
			photo.LargeURL = first(p.URLO, p.URLL, staticImageURL(p, "b"), photo.SmallURL)
			photo.Width = parseInt(string(p.WidthO), string(p.WidthL), string(p.WidthM))
			photo.Height = parseInt(string(p.HeightO), string(p.HeightL), string(p.HeightM))
			if photo.Location == "" && (string(p.Latitude) != "0" || string(p.Longitude) != "0") {
				photo.Location = strings.Trim(string(p.Latitude)+", "+string(p.Longitude), ", ")
			}
			_ = c.addEXIF(ctx, token, &photo)
			out = append(out, photo)
		}
		if page >= int(response.Photoset.Pages) || response.Photoset.Pages == 0 {
			return out, nil
		}
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func staticImageURL(photo photoJSON, size string) string {
	if photo.ID == "" || photo.Server == "" || photo.Secret == "" {
		return ""
	}
	return "https://live.staticflickr.com/" + url.PathEscape(photo.Server) + "/" + url.PathEscape(photo.ID) + "_" + url.PathEscape(photo.Secret) + "_" + size + ".jpg"
}
func (c *Client) PhotoInfo(ctx context.Context, token Token, id string) (Photo, bool, error) {
	var response struct {
		Photo struct {
			ID                 string
			Owner              struct{ NSID string }
			Title, Description contentValue
			Visibility         struct {
				IsPublic int `json:"ispublic"`
			}
			URLs struct {
				URL []contentValue `json:"url"`
			} `json:"urls"`
			Location struct {
				Locality contentValue `json:"locality"`
				County   contentValue `json:"county"`
				Region   contentValue `json:"region"`
				Country  contentValue `json:"country"`
			} `json:"location"`
		} `json:"photo"`
	}
	if err := c.call(ctx, token, "flickr.photos.getInfo", url.Values{"photo_id": {id}}, &response); err != nil {
		return Photo{}, false, err
	}
	page := ""
	if len(response.Photo.URLs.URL) > 0 {
		page = response.Photo.URLs.URL[0].Content
	}
	parts := []string{}
	for _, value := range []string{response.Photo.Location.Locality.Content, response.Photo.Location.County.Content, response.Photo.Location.Region.Content, response.Photo.Location.Country.Content} {
		if value != "" && (len(parts) == 0 || parts[len(parts)-1] != value) {
			parts = append(parts, value)
		}
	}
	return Photo{ID: id, Title: response.Photo.Title.Content, Description: response.Photo.Description.Content, PageURL: page, Location: strings.Join(parts, ", ")}, response.Photo.Visibility.IsPublic == 1, nil
}

func (c *Client) addEXIF(ctx context.Context, token Token, p *Photo) error {
	var response struct {
		Photo struct {
			Camera string
			EXIF   []struct {
				Tag   string
				Label string
				Raw   contentValue
			} `json:"exif"`
		} `json:"photo"`
	}
	if err := c.call(ctx, token, "flickr.photos.getExif", url.Values{"photo_id": {p.ID}}, &response); err != nil {
		return err
	}
	p.Camera = response.Photo.Camera
	for _, e := range response.Photo.EXIF {
		switch strings.ToLower(e.Tag) {
		case "lensmodel", "lens":
			p.Lens = e.Raw.Content
		case "exposuretime":
			p.Exposure = e.Raw.Content
		case "fnumber":
			p.Aperture = normalizeAperture(e.Raw.Content)
		case "iso":
			p.ISO = e.Raw.Content
		case "focallength":
			p.FocalLength = e.Raw.Content
		}
	}
	return nil
}
func normalizeAperture(v string) string {
	if v != "" && !strings.HasPrefix(strings.ToLower(v), "f/") {
		return "f/" + v
	}
	return v
}
