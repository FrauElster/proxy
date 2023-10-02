package proxy

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	goProxy "golang.org/x/net/proxy"
)

type stealthTransport struct {
	// Transport is the underlying transport used by the stealth transport
	Transport http.RoundTripper
	// userAgents is a list of user agents used by the stealth transport
	// if the list is empty, the stealth transport will not set a user agent
	userAgents []string

	// minDelay and maxDelay are the minimum and maximum delay between requests
	// the actual delay will be a random value between min and max
	// if minDelay or maxDelay are 0, the stealth transport will not delay requests
	minDelay    time.Duration
	maxDelay    time.Duration
	lastRequest time.Time

	// socks5Proxy is the SOCKS5 proxy used by the stealth transport
	// if socks5Proxy is empty, the stealth transport will not use a SOCKS5 proxy
	socks5Proxy       string
	socksAuth         *goProxy.Auth
	socks5Initialized bool

	// compression is true if the stealth transport will compress requests and decompress responses
	// if the request is already compressed, the stealth transport will not compress it again, and will not decompress the response
	compression bool
}

type StealthOption func(*stealthTransport)

// WithCompression enables compression for the stealth transport
var WithCompression = func(s *stealthTransport) {
	s.compression = true
}

// WithSocks5 sets the SOCKS5 proxy used by the stealth transport
func WithSocks5(proxyAddr string, auth *goProxy.Auth) StealthOption {
	return func(s *stealthTransport) {
		s.socks5Proxy = proxyAddr
		s.socksAuth = auth
	}
}

// WithUserAgents the stealth transport will randomly choose one of the given user agents
// the most common user agents can be found in CommonUserAgents
func WithUserAgents(agents ...string) StealthOption {
	return func(s *stealthTransport) {
		s.userAgents = agents
	}
}

// WithDelay sets the minimum and maximum delay between requests
// the actual delay will be a random value between min and max
func WithDelay(min, max time.Duration) StealthOption {
	return func(s *stealthTransport) {
		s.minDelay = min
		s.maxDelay = max
	}
}

func NewStealthTransport(opts ...StealthOption) *stealthTransport {
	t := &stealthTransport{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// RoundTrip implements the http.RoundTripper interface
func (t *stealthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// set a random user agent if one is not already set
	if len(t.userAgents) > 0 {
		randomUserAgent := t.userAgents[rand.Intn(len(t.userAgents))]
		addHeaderIfNotExists(req, "User-Agent", randomUserAgent)
	}

	// add basic headers if they are not already set
	addHeaderIfNotExists(req, "Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
	addHeaderIfNotExists(req, "Accept", "*/*")
	addHeaderIfNotExists(req, "Connection", "keep-alive")
	addHeaderIfNotExists(req, "Cache-Control", "no-cache")
	addHeaderIfNotExists(req, "Pragma", "no-cache")
	addHeaderIfNotExists(req, "DNT", "1")

	// add compression header
	hadCompression := req.Header.Get("Accept-Encoding") != ""
	if t.compression && !hadCompression {
		slog.Info("accepting compression")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}

	// initialize the SOCKS5 proxy if one is set
	if t.socks5Proxy != "" && !t.socks5Initialized {
		dialer, err := goProxy.SOCKS5("tcp", t.socks5Proxy, t.socksAuth, goProxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SOCKS5 proxy: %w", err)
		}

		currentTransport := t.Transport.(*http.Transport)
		currentTransport.Dial = dialer.Dial
		t.socks5Initialized = true
	}

	// delay the request if necessary
	if t.minDelay > 0 && t.maxDelay > 0 {
		actualDelay := time.Since(t.lastRequest)
		wantDelay := rand.Int63n(int64(t.maxDelay-t.minDelay)) + int64(t.minDelay)
		if actualDelay < time.Duration(wantDelay) {
			time.Sleep(time.Duration(wantDelay) - actualDelay)
		}
	}

	t.lastRequest = time.Now()
	res, resErr := t.Transport.RoundTrip(req)
	if resErr != nil {
		return nil, resErr
	}

	// decompress
	if t.compression && !hadCompression && res.Header.Get("Content-Encoding") != "" {
		slog.Info("decompressing")
		err := decompressResponse(res)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func addHeaderIfNotExists(req *http.Request, key, value string) {
	if req.Header.Get(key) == "" {
		req.Header.Set(key, value)
	}
}

var CommonUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537",
	"Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; AS; rv:11.0) like Gecko",
	"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.81 Safari/537.36 Edge/16.16299",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_3) AppleWebKit/602.4.8 (KHTML, like Gecko) Version/10.0.3 Safari/602.4.8",
	"Mozilla/5.0 (Windows NT 10.0; WOW64; Trident/7.0; AS; rv:11.0) like Gecko",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/46.0.2486.0 Safari/537.36 Edge/13.10586",
	"Mozilla/5.0 (Windows NT 6.1; Trident/7.0; AS; rv:11.0) like Gecko",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/57.0.2987.133 Safari/537.36 Edge/16.16299",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/57.0.2987.133 Safari/537.36",
	"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:54.0) Gecko/20100101 Firefox/54.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:68.0) Gecko/20100101 Firefox/68.0",
	"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.90 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.132 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.120 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.97 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.132 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.122 Safari/537.36",
}
