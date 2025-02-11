package fhttp

import (
	"github.com/sleeyax/gotcha"
	fhttpPkg "github.com/useflyent/fhttp"
	"net/http"
)

type Adapter struct {
	// Optional fhttp Transport options.
	Transport *fhttpPkg.Transport
}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) DoRequest(options *gotcha.Options) (*gotcha.Response, error) {
	req := &fhttpPkg.Request{
		Method: options.Method,
		URL:    options.FullUrl,
		Header: fhttpPkg.Header(options.Headers),
		Body:   options.Body,
	}

	if a.Transport == nil {
		a.Transport = fhttpPkg.DefaultTransport.(*fhttpPkg.Transport)
	}

	if options.Proxy != nil {
		a.Transport.Proxy = fhttpPkg.ProxyURL(options.Proxy)
	}

	if options.CookieJar != nil {
		for _, cookie := range options.CookieJar.Cookies(options.FullUrl) {
			req.AddCookie(&fhttpPkg.Cookie{Name: cookie.Name, Value: cookie.Value})
		}
	}

	res, err := a.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	r := toResponse(req, res)

	if options.CookieJar != nil {
		if rc := r.Cookies(); len(rc) > 0 {
			options.CookieJar.SetCookies(options.FullUrl, rc)
		}
	}

	return &gotcha.Response{r, options.UnmarshalJson}, nil
}

// toResponse converts fhttp response to an original http response.
func toResponse(req *fhttpPkg.Request, res *fhttpPkg.Response) *http.Response {
	return &http.Response{
		Status:           res.Status,
		StatusCode:       res.StatusCode,
		Proto:            res.Proto,
		ProtoMajor:       res.ProtoMajor,
		ProtoMinor:       res.ProtoMinor,
		Header:           http.Header(res.Header),
		Body:             res.Body,
		ContentLength:    res.ContentLength,
		TransferEncoding: res.TransferEncoding,
		Close:            res.Close,
		Uncompressed:     res.Uncompressed,
		Trailer:          http.Header(res.Trailer),
		Request:          toRequest(req),
		TLS:              res.TLS,
	}
}

// toResponse converts a fhttp request to an original http response.
func toRequest(req *fhttpPkg.Request) *http.Request {
	return &http.Request{
		Method: req.Method,
		URL:    req.URL,
		Header: http.Header(req.Header),
		Body:   req.Body,
	}
}
