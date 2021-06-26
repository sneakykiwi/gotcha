package gotcha

import "net/http"

type RetryFunc func(updatedOptions *Options) (*http.Response, error)

type BeforeRequestHook func(*Options)

type BeforeRedirectHook func(*Options, *http.Response)

type AfterResponseHook func(response *http.Response, retry RetryFunc) (*http.Response, error)

type BeforeRetryHook func(options *Options, error error, retryCount int)

type InitHook func(*Options)

type Hooks struct {
	// Called with plain Options, right before their normalization.
	Init []InitHook

	// Called with normalized Options.
	// Gotcha will make no further changes to the Options before it is sent to the Adapter.
	//
	// Note that changing Options.Json or Options.Form has no effect on the request,
	// you should change Options.Body instead and (if needed) update the options.headers accordingly.
	BeforeRequest []BeforeRequestHook

	// Called with normalized request options and the redirect response.
	// Gotcha will make no further changes to the request.
	BeforeRedirect []BeforeRedirectHook

	// Called with normalized request options, the error and the retry count.
	// Gotcha will make no further changes to the request.
	BeforeRetry []BeforeRetryHook

	// Called with response and a retry function.
	// Calling the retry function will trigger BeforeRetry hooks.
	//
	// Each function should return the (modified) response.
	AfterResponse []AfterResponseHook
}
