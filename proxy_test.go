package proxy_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"

	goProxy "golang.org/x/net/proxy"

	"github.com/FrauElster/proxy"
	"github.com/FrauElster/proxy/internal"
	"github.com/FrauElster/proxy/stats"
	"github.com/FrauElster/proxy/stealth"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

var GithubTarget = proxy.Target{BaseUrl: "https://github.com", Prefix: "/github/"}
var WikipediaTarget = proxy.Target{BaseUrl: "https://wikipedia.org", Prefix: "/wikipedia/"}

func TestProxy(t *testing.T) {
	t.Run("Check if the response body is the same", func(t *testing.T) {
		proxy, err := proxy.NewProxy([]proxy.Target{GithubTarget})
		require.NoError(t, err)
		startProxy(t, proxy)
		defer stopServer(t, proxy)

		originalUrl := "https://github.com/FrauElster"
		originalBody := getBody(t, originalUrl)

		proxyUrl := internal.JoinUrl(proxy.Addr(), GithubTarget.Prefix, "FrauElster")
		proxyBody := getBody(t, proxyUrl)

		// due to rewritten URLs the body is not the same
		require.True(t, isLengthApproximatelyEqual(originalBody, proxyBody, 10), "original: %d\nproxy: %d", len(originalBody), len(proxyBody))
	})

	t.Run("check with SOCKS5 proxy", func(t *testing.T) {
		proxy, err := proxy.NewProxy([]proxy.Target{GithubTarget}, proxy.WithTransport(mustSocksTransport(t)))
		require.NoError(t, err)
		startProxy(t, proxy)
		defer stopServer(t, proxy)

		originalUrl := "https://github.com/FrauElster"
		originalBody := getBody(t, originalUrl)

		proxyUrl := internal.JoinUrl(proxy.Addr(), GithubTarget.Prefix, "FrauElster")
		proxyBody := getBody(t, proxyUrl)

		// due to rewritten URLs the body is not the same
		require.Truef(t, isLengthApproximatelyEqual(originalBody, proxyBody, 10), "original: %d\nproxy: %d", len(originalBody), len(proxyBody))
	})
}

func isLengthApproximatelyEqual(a, b string, marginPercent float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false // Handle empty strings
	}

	// Calculate the lower and upper bounds for the length of "b" based on the margin percent
	minLengthB := float64(len(b)) * (1 - marginPercent/100)
	maxLengthB := float64(len(b)) * (1 + marginPercent/100)

	// Check if the length of "a" falls within the bounds
	return float64(len(a)) >= minLengthB && float64(len(a)) <= maxLengthB
}

func getBody(t *testing.T, url string) string {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Error requesting %s", url)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err, "Error reading response body. Status: %s", res.Status)
	return string(body)
}

func startProxy(t *testing.T, proxy *proxy.Proxy) {
	go func() {
		err := proxy.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	}()
}

func stopServer(t *testing.T, proxy *proxy.Proxy) {
	err := proxy.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func mustSocksTransport(t *testing.T) *stealth.StealthTransport {
	err := godotenv.Load()
	require.NoError(t, err)

	socksAddr := os.Getenv("SOCKS5_PROXY")
	user := os.Getenv("SOCKS5_USER")
	pass := os.Getenv("SOCKS5_PASS")
	transport := stealth.NewStealthTransport(stealth.WithSocks5(socksAddr, &goProxy.Auth{
		User:     user,
		Password: pass,
	}))
	return transport
}

func XTestRun(t *testing.T) {
	stats := stats.NewStatServer()
	stats.RegisterTarget(GithubTarget)
	stats.RegisterTarget(WikipediaTarget)

	proxy, err := proxy.NewProxy([]proxy.Target{GithubTarget, WikipediaTarget}, proxy.WithTransport(mustSocksTransport(t)), proxy.WithPort(8080))
	require.NoError(t, err)

	go func() {
		err := stats.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	startProxy(t, proxy)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}
