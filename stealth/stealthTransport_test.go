package stealth

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-socks5"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	goProxy "golang.org/x/net/proxy"
)

func TestStealthClient(t *testing.T) {
	t.Run("Test User Agents", func(t *testing.T) {
		// Create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent := r.Header.Get("User-Agent")
			require.NotEmpty(t, userAgent, "User-Agent should not be empty")
			require.Contains(t, CommonUserAgents, userAgent, "User-Agent should be one of the common user agents")
		}))
		defer server.Close()

		transport := NewStealthTransport(WithUserAgents(CommonUserAgents...))
		c := &http.Client{Transport: transport}

		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)
		c.Do(req)
	})

	t.Run("Test Delay", func(t *testing.T) {
		// Create mock HTTP server
		var lastRequest time.Time
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			if !lastRequest.IsZero() {
				require.True(t, now.Sub(lastRequest) >= time.Second, "Requests should be delayed by at least 1 second")
			}
			lastRequest = now
		}))
		defer server.Close()

		transport := NewStealthTransport(WithDelay(time.Second, 2*time.Second))
		c := &http.Client{Transport: transport}

		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)
		c.Do(req)
		c.Do(req)
	})

	t.Run("Test SOCKS5", func(t *testing.T) {
		// Create a mock SOCKS5 server
		hitSocks := false
		socksAddr := "127.0.0.1:9090"
		server, err := socks5.New(&socks5.Config{
			Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				hitSocks = true
				return net.Dial(network, addr)
			},
		})
		require.NoError(t, err)
		go server.ListenAndServe("tcp", socksAddr)
		time.Sleep(time.Second)

		// Create a mock HTTP server
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			socksHost := strings.Split(r.RemoteAddr, ":")[0]
			reqIP := strings.Split(r.RemoteAddr, ":")[0]
			require.Equal(t, socksHost, reqIP, "Request should be proxied through the SOCKS5 server")
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		transport := NewStealthTransport(WithSocks5(socksAddr, nil))
		c := &http.Client{Transport: transport}

		req, err := http.NewRequest("GET", mockServer.URL, nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		require.True(t, hitSocks, "Should have hit the SOCKS5 server")
	})

	t.Run("Test SOCKS5 with NordVPN", func(t *testing.T) {
		c := &http.Client{Transport: mustSocksTransport(t)}

		req, err := http.NewRequest("GET", "https://www.github.com", nil)
		require.NoError(t, err)
		resp, err := c.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func mustSocksTransport(t *testing.T) *StealthTransport {
	err := godotenv.Load("../.env")
	require.NoError(t, err)

	socksAddr := os.Getenv("SOCKS5_PROXY")
	user := os.Getenv("SOCKS5_USER")
	pass := os.Getenv("SOCKS5_PASS")
	transport := NewStealthTransport(WithSocks5(socksAddr, &goProxy.Auth{
		User:     user,
		Password: pass,
	}))
	return transport
}
