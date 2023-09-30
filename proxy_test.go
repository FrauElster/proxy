package proxy

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProxy(t *testing.T) {
	t.Run("Check if the response body is the same", func(t *testing.T) {
		proxy, err := NewProxy([]Target{GithubTarget})
		require.NoError(t, err)
		startProxy(t, proxy)
		defer stopServer(t, proxy)

		originalUrl := "https://github.com/FrauElster"
		originalBody := getBody(t, originalUrl)

		proxyUrl := joinUrl(proxy.Addr(), GithubTarget.Prefix, "FrauElster")
		proxyBody := getBody(t, proxyUrl)

		// due to rewritten URLs the body is not the same
		require.True(t, isLengthApproximatelyEqual(originalBody, proxyBody, 10), "original: %d\nproxy: %d", len(originalBody), len(proxyBody))
	})

	t.Run("check with SOCKS5 proxy", func(t *testing.T) {
		proxy, err := NewProxy([]Target{GithubTarget}, WithTransport(mustSocksTransport(t)))
		require.NoError(t, err)
		startProxy(t, proxy)
		defer stopServer(t, proxy)

		originalUrl := "https://github.com/FrauElster"
		originalBody := getBody(t, originalUrl)

		proxyUrl := joinUrl(proxy.Addr(), GithubTarget.Prefix, "FrauElster")
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

func startProxy(t *testing.T, proxy *proxy) {
	go func() {
		err := proxy.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	}()
}

func stopServer(t *testing.T, proxy *proxy) {
	err := proxy.Shutdown(context.Background())
	if err != nil {
		t.Error(err)
	}
}

var _githubUrl, _ = url.Parse("https://github.com")
var GithubTarget = Target{
	BaseUrl: *_githubUrl,
	Prefix:  "/github/",
}
