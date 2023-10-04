package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/FrauElster/proxy/internal"
	"github.com/PuerkitoBio/goquery"
)

type Target struct {
	BaseUrl string
	Prefix  string
	// PreRequest can be used to manipulate the http.Request
	PreRequest func(*http.Request) *http.Request
	// PostRequest can be used to manipulate the http.Response
	// if the request failed, *http.Response will be nil and the returned value will be ignored
	PostRequest func(*http.Response) *http.Response
}

type ProxyOption func(*Proxy)

// WithSsl enables SSL for the proxy server
// ListenAndServe will use http.ListenAndServeTLS instead of http.ListenAndServe
func WithSsl(cert tls.Certificate) ProxyOption {
	return func(p *Proxy) { p.cert = &cert }
}

// WithTransport sets the transport used by the proxy server
func WithTransport(transport http.RoundTripper) ProxyOption {
	return func(p *Proxy) { p.transport = transport }
}

func WithPort(port int) ProxyOption {
	return func(p *Proxy) { p.port = port }
}

type Proxy struct {
	targets   map[string]Target
	transport http.RoundTripper
	server    *http.Server
	port      int

	addr *url.URL
	cert *tls.Certificate
}

func NewProxy(targets []Target, opts ...ProxyOption) (*Proxy, error) {
	targetMap := make(map[string]Target)
	for i, target := range targets {
		if !strings.HasPrefix(target.Prefix, "/") {
			target.Prefix = "/" + target.Prefix
		}

		_, err := url.Parse(target.BaseUrl)
		if err != nil {
			return nil, fmt.Errorf("error parsing target URL %s: %w", target.BaseUrl, err)
		}

		targetMap[target.Prefix] = targets[i]
	}

	p := &Proxy{
		targets:   targetMap,
		transport: http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(p)
	}

	p.addr = &url.URL{Scheme: "http", Host: fmt.Sprintf("0.0.0.0:%d", p.port)}

	if p.cert != nil {
		p.addr.Scheme = "https"
	}

	return p, nil
}

// ListenAndServe starts the proxy server
// It blocks until the server is shut down
// If the proxy server was started with WithSsl, it will use http.ListenAndServeTLS instead of http.ListenAndServe
func (p *Proxy) ListenAndServe() (err error) {
	// start listener (so we can get the actual port, even if it was chosen by the OS)
	listener, err := net.Listen("tcp", p.addr.Host)
	if err != nil {
		return fmt.Errorf("error starting listener: %w", err)
	}
	defer listener.Close()
	p.addr.Host = listener.Addr().String()

	// build server
	mux := http.NewServeMux()
	for path, target := range p.targets {
		target := target
		mux.HandleFunc(path, p.forwardRequest(&target))
	}
	p.server = &http.Server{
		Addr:    p.addr.Host,
		Handler: mux,
	}

	// start server
	if p.cert == nil {
		return p.server.Serve(listener)
	}

	// start TLS server
	p.server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*p.cert}}
	return p.server.ServeTLS(listener, "", "")
}

func (p *Proxy) Shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

func (p *Proxy) Addr() string {
	return p.addr.String()
}

func (p *Proxy) forwardRequest(target *Target) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		newReq, err := buildRequest(r, *target)
		if err != nil {
			slog.Warn("Error constructing new request", "err", err)
			http.Error(w, "Error constructing new request", http.StatusBadGateway)
			return
		}

		// Send the new request
		if target.PreRequest != nil {
			newReq = target.PreRequest(newReq)
		}
		client := &http.Client{Transport: p.transport}
		resp, err := client.Do(newReq)
		if target.PostRequest != nil {
			resp = target.PostRequest(resp)
		}
		if err != nil {
			slog.Warn("Error forwarding request", "err", err)
			http.Error(w, "Error forwarding request", http.StatusBadGateway)
			return
		}

		// If it's an OPTIONS request (a preflight CORS request), respond with OK
		if r.Method == http.MethodOptions {
			// Add CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			w.WriteHeader(http.StatusOK)
			return
		}

		err = p.copyResponse(resp, w, *target)
		if err != nil {
			slog.Warn("Error copying response", "err", err)
			http.Error(w, "Error copying response", http.StatusBadGateway)
			return
		}
	}
}

func (p *Proxy) copyResponse(resp *http.Response, w http.ResponseWriter, target Target) error {
	// Copy the headers from the target server to the original response writer
	copyHeaders(resp, w)

	// we have to decompress the response before we can copy the body
	encoding := resp.Header.Get("Content-Encoding")
	if encoding != "" {
		err := internal.DecompressResponse(resp)
		if err != nil {
			return fmt.Errorf("error decompressing response body: %w", err)
		}
	}
	defer resp.Body.Close()

	// Copy the body from the target server to the original response writer
	newBody, err := p.copyBody(resp, target)
	if err != nil {
		return fmt.Errorf("error copying response body: %w", err)
	}

	// compress the response again
	if encoding != "" {
		newBody, err = internal.CompressBody(newBody, internal.SupportedCompression(encoding))
		if err != nil {
			return fmt.Errorf("error compressing response body: %w", err)
		}
		w.Header().Set("Content-Encoding", encoding)
	}

	w.WriteHeader(resp.StatusCode)
	w.Write([]byte(newBody))
	return nil
}

func copyHeaders(resp *http.Response, w http.ResponseWriter) {
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func (p *Proxy) copyBody(resp *http.Response, target Target) ([]byte, error) {
	// if not HTML just copy the body
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		return io.ReadAll(resp.Body)
	}

	// parse HTML
	originalBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading (decompressed) response body")
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(originalBody))
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML content")
	}

	// Replace all links and script tags with the proxy URL
	document.Find("a[href], img[src], link[href], script[src]").Each(func(index int, element *goquery.Selection) {
		for _, attr := range []string{"href", "src"} {
			if val, exists := element.Attr(attr); exists {
				isDynamic := strings.HasPrefix(val, "/")
				isOnOriginalHost := strings.HasPrefix(val, target.BaseUrl)

				url := p.addr
				url.Path = internal.JoinUrl(target.Prefix, strings.TrimPrefix(val, target.BaseUrl))
				if isDynamic || isOnOriginalHost {
					element.SetAttr(attr, url.String())
				}
			}
		}
	})

	// parse back to HTML
	newBody, err := document.Html()
	if err != nil {
		return nil, fmt.Errorf("error getting modified HTML content")
	}

	return []byte(newBody), nil
}

func buildRequest(originalReq *http.Request, target Target) (*http.Request, error) {
	// Create a new URL from the base URL of the target server and the path from the original request
	targetAsUrl, err := url.Parse(target.BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing target URL")
	}
	newURL := *originalReq.URL
	newURL.Scheme = targetAsUrl.Scheme
	newURL.Host = targetAsUrl.Host
	newURL.Path = strings.TrimPrefix(newURL.Path, target.Prefix)

	// Create a new request with the original method, the new URL, and the original body
	bodyBytes, err := io.ReadAll(originalReq.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body")
	}
	newReq, err := http.NewRequest(originalReq.Method, newURL.String(), io.NopCloser(bytes.NewReader(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("error creating new request")
	}

	// Copy the original headers to the new request
	for name, values := range originalReq.Header {
		for _, value := range values {
			newReq.Header.Add(name, value)
		}
	}

	newReq.Close = true
	return newReq, nil
}
