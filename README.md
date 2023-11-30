# Proxy

<p align="center">
  <img src="assets/logo.png" alt="Project Logo" width="800">
</p>

[![Code Coverage](https://img.shields.io/badge/coverage-58%25-brightgreen)](#)
[![Last Updated](https://img.shields.io/badge/updated-2023.11.29-brightgreen)](#)

This project provides a proxy server that can proxy multiple websites with a custom http.Transport.

## _Install_

`go get github.com/FrauElster/proxy`

## _Why?_ 

I have a automated browser which crawles websites, and I wanted it to use a VPN without the system its running on needing to connect to one. Since I dont have control over the http transport the browser uses, I had to come up with this proxy.

## _What exactly can it do?_ 

It is basically a man in the middle between your requesting client and the actual website.
Lets say you want to `GET www.google.com/search?q=hello+world`, you would setup the proxy and send you request to `GET localhost:8080/google/search?q=hello+world`. The proxy would request the website over a custom http.Transport (e.g. using SOCKS5), swap all URL that resolve to `www.google.com` with `localhost:8080/google` and forwards the response to the client.

## _How to use?_

```go
import (
  goproxy "golang.org/x/net/proxy"
)

func main() {
  // build a custom transport, this can be any http.RoundTripper,
  socksAddr := os.Getenv("SOCKS5_PROXY")
	user := os.Getenv("SOCKS5_USER")
	pass := os.Getenv("SOCKS5_PASS")
	transport := proxy.NewStealthTransport(
    proxy.WithSocks5(socksAddr, &goproxy.Auth{User: user, Password: pass}), 
    proxy.WithUserAgents(proxy.CommonUserAgents...)
  )

  // define website to forward to
  targets := []proxy.Target({BaseUrl: "https://www.github.com", Prefix:  "/github/"})

  // build proxy
  p, err := proxy.NewProxy(targets, 
    proxy.WithTransport(transport), 
    proxy.WithPort(8080)
  )
  if err != nil {
    panic(err)
  }

  // start the server
  err := proxy.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
```

## _Overwatch it_

The package comes with a separate stats server, that can be used like that:

```go
func main() {
  // define targets
	targetOne := Target{BaseUrl: "https://example.com", Prefix: "/example/"}
  targetTwo := Target{BaseUrl: "https://github.com", Prefix: "/github/"}
	
  // start stats server
  stats := stats.NewStatServer(stats.WithPort(8081))
  stats.RegisterTarget(targetOne)
  stats.RegisterTarget(targetTwo)
	go func() {
		err := stats.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

  // start proxy
  p, err := proxy.NewProxy([]proxy.Target{targetOne, targetTwo}, proxy.WithPort(8080))
  if err != nil {
    panic(err)
  }
  go func() {
		err := p.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

  // wait for ctrl+c
  sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}
```

## _CORS_

Some Browser wont allow the forwarding from a secure (https) website over a unsecure connection (http).
Therefore, this package comes with a function to generate a self-signed cert, which can be applied to the proxy:

```go
func main() {
  // define targets
	targetOne := Target{BaseUrl: "https://example.com", Prefix: "/example/"}
  targetTwo := Target{BaseUrl: "https://github.com", Prefix: "/github/"}

  // generate cert
  sslCert, err := proxy.GenerateSslCerts("Hans Maier GmbH")
	if err != nil {
    panic(err)
  }

  // start proxy
	proxy, err := proxy.NewProxy(
		[]proxy.Target{targetOne, targetTwo}
		proxy.WithSsl(sslCert),
	)
  // ListenAndServe will automatically use ServeTLS
  err := p.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
```