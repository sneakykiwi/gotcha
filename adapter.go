package gotcha

import (
	"net/http"
)

type Adapter interface {
	// DoRequest is a custom function that will be used by gotcha to make the actual request.
	DoRequest(options *Options) (*http.Response, error)
}

// RequestAdapter is a basic implementation of Adapter.
type RequestAdapter struct {
	// RoundTripper is a http.RoundTripper that will be used to do the request.
	//
	// Defaults to http.DefaultTransport.
	RoundTripper http.RoundTripper

	// Request is the http.Request to send.
	//
	// The Request will be derived from Options when unspecified.
	Request *http.Request
}

func (ra *RequestAdapter) DoRequest(options *Options) (*http.Response, error) {
	if ra.Request == nil {
		ra.Request = &http.Request{
			Method: options.Method,
			URL:    options.Url,
			Header: options.Headers,
			Body:   options.Body,
		}
	}

	if ra.RoundTripper == nil {
		ra.RoundTripper = http.DefaultTransport
	}

	res, err := ra.RoundTripper.RoundTrip(ra.Request)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// mockAdapter is only used for testing.
type mockAdapter struct {
	OnCalledDoRequest func(*Options)
}

func (ma *mockAdapter) DoRequest(options *Options) (*http.Response, error) {
	ma.OnCalledDoRequest(options)
	return &http.Response{StatusCode: 200}, nil
}
