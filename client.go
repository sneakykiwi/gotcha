package gotcha

import (
	bytes2 "bytes"
	"github.com/sleeyax/gotcha/internal/utils"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	options *Options
}

// NewClient creates a new HTTP client based on default Options which can be extended.
func NewClient(options *Options) (*Client, error) {
	client := &Client{NewDefaultOptions()}
	return client.Extend(options)
}

// Extend returns a new Client with merged Options.
func (c *Client) Extend(options *Options) (*Client, error) {
	opts, err := c.options.Extend(options)
	if err != nil {
		return nil, err
	}
	return &Client{opts}, nil
}

func (c *Client) DoRequest(method string, url string, options ...*Options) (*http.Response, error) {
	for _, option := range options {
		var err error
		c.options, err = c.options.Extend(option)
		if err != nil {
			return nil, err
		}
	}

	u, err := utils.GetFullUrl(c.options.PrefixURL, url)
	if err != nil {
		return nil, err
	}

	c.options.FullUrl = u
	c.options.Method = method
	c.options.URI = url

	if sp := c.options.SearchParams; len(sp) != 0 {
		c.options.FullUrl.RawQuery = sp.EncodeWithOrder()
	}

	c.ParseBody()
	defer c.CloseBody()

	retry := func(res *http.Response, err error) (*http.Response, error) {
		timeout, e := c.getTimeout(res)
		if e != nil {
			return nil, e
		}
		timeout = c.options.RetryOptions.CalculateTimeout(c.options.retries, c.options.RetryOptions, timeout, err)
		time.Sleep(timeout)
		c.options.retries++
		return c.DoRequest(method, url)
	}

	res, err := c.options.Adapter.DoRequest(c.options)
	if err != nil {
		if c.options.Retry && c.options.retries < c.options.RetryOptions.Limit && utils.StringArrayContains(c.options.RetryOptions.ErrorCodes, err.Error()) {
			return retry(res, err)
		}
		return nil, err
	}

	if c.options.Retry && utils.IntArrayContains(c.options.RetryOptions.StatusCodes, res.StatusCode) && utils.StringArrayContains(c.options.RetryOptions.Methods, method) {
		if c.options.retries >= c.options.RetryOptions.Limit {
			return res, NewMaxRetriesExceededError()
		}
		return retry(res, nil)
	}

	if c.options.FollowRedirect && res.Header.Get("location") != "" && utils.IntArrayContains(RedirectStatusCodes, res.StatusCode) {
		// we don't care about the response since we're redirecting
		res.Body.Close()

		if c.options.RedirectOptions.Limit != 0 && len(c.options.redirectUrls) >= c.options.RedirectOptions.Limit {
			return nil, NewMaxRedirectsExceededError(len(c.options.redirectUrls))
		}

		if c.options.RedirectOptions.RewriteMethods || (res.StatusCode == 303 && c.options.Method != "GET" && c.options.Method != "HEAD") {
			c.options.Method = "GET"
			c.CloseBody()
			c.options.Headers.Del("content-length")
			c.options.Headers.Del("content-type")
		}

		currentUrl := c.options.FullUrl
		redirectUrl, err := utils.GetFullUrl(currentUrl.String(), res.Header.Get("location"))
		if err != nil {
			return nil, err
		}

		// remove sensitive headers when redirecting to a different domain
		if redirectUrl.Hostname() != currentUrl.Hostname() || redirectUrl.Port() != currentUrl.Port() {
			c.options.Headers.Del("host")
			c.options.Headers.Del("cookie")
			c.options.Headers.Del("authorization")
		}

		c.updateRequestCookies(res)
		c.options.PrefixURL = ""
		c.options.redirectUrls = append(c.options.redirectUrls, redirectUrl)

		// TODO: call redirect hook

		return c.DoRequest(c.options.Method, redirectUrl.String())
	}

	return res, nil
}

func (c *Client) getTimeout(response *http.Response) (time.Duration, error) {
	retryAfter := strings.TrimSpace(response.Header.Get("retry-after"))

	// response header doesn't specify timeout, default to request timeout
	if retryAfter == "" || c.options.RetryOptions.RetryAfter == false {
		return c.options.Timeout, nil
	}

	// retryAfter is <delay-seconds>
	if delay, err := strconv.Atoi(retryAfter); err == nil {
		return time.Second * time.Duration(delay), nil
	}

	// retryAfter is <http-date>
	dateTime, err := http.ParseTime(retryAfter)
	if err == nil {
		return dateTime.Sub(time.Now()), nil
	}
	return 0, err
}

// updateRequestCookies synchronizes request cookies that were manually set through the Cookie http.Header
// with corresponding cookies from a http.Response after redirect.
//
// If CookieJar is present and there was some initial cookies provided
// via the request header, then we may need to alter the initial
// cookies as we follow redirects since each redirect may end up
// modifying a pre-existing cookie.
//
// Since cookies already set in the request header do not contain
// information about the original domain and path, the logic below
// assumes any new set cookies override the original cookie
// regardless of domain or path.
func (c *Client) updateRequestCookies(response *http.Response) {
	cookies := c.getRequestCookies()
	if c.options.CookieJar != nil && cookies != nil {
		// changed denotes whether or not a response cookie has a different value than a request cookie
		var changed bool
		for _, cookie := range response.Cookies() {
			if _, ok := cookies[cookie.Name]; ok {
				delete(cookies, cookie.Name)
				changed = true
			}
		}
		if changed {
			c.options.Headers.Del("Cookie")
			var cks []string
			for _, cs := range cookies {
				for _, cookie := range cs {
					cks = append(cks, cookie.Name+"="+cookie.Value)
				}
			}
			c.options.Headers.Set("Cookie", strings.Join(cks, "; "))
		}
	}
}

// getRequestCookies returns the cookies that were manually set in Options.Headers.
func (c *Client) getRequestCookies() map[string][]*http.Cookie {
	var cookies map[string][]*http.Cookie

	if c.options.CookieJar != nil && c.options.Headers.Get("cookie") != "" {
		cookies = make(map[string][]*http.Cookie)
		req := http.Request{Header: c.options.Headers}
		for _, c := range req.Cookies() {
			cookies[c.Name] = append(cookies[c.Name], c)
		}
	}

	return cookies
}

// CloseBody clears the Body, Form and Json fields.
func (c *Client) CloseBody() {
	if c.options.Body != nil {
		c.options.Body.Close()
	}
	c.options.Body = nil
	c.options.Form = nil
	c.options.Json = nil
}

// ParseBody parses Form or Json (in that order) into Content.
func (c *Client) ParseBody() error {
	if len(c.options.Form) != 0 {
		encoded := c.options.Form.EncodeWithOrder()
		c.options.Body = io.NopCloser(strings.NewReader(encoded))
		return nil
	} else if j := c.options.Json; len(j) != 0 {
		bytes, err := c.options.MarshalJson(j)
		if err != nil {
			return err
		}
		c.options.Body = io.NopCloser(bytes2.NewReader(bytes))
		return nil
	}
	return nil
}

func (c *Client) Get(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("GET", url, options...)
}

func (c *Client) Post(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("POST", url, options...)
}

func (c *Client) Update(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("UPDATE", url, options...)
}

func (c *Client) Patch(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("PATCH", url, options...)
}

func (c *Client) Delete(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("DELETE", url, options...)
}

func (c *Client) Head(url string, options ...*Options) (*http.Response, error) {
	return c.DoRequest("HEAD", url, options...)
}
