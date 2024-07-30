package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

var (
	ErrInvalidProxy = errors.New("invalid proxy")
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

func (c *Config) FromURL(rawURL string) error {
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	c.Host = proxyURL.Hostname()
	c.Port = proxyURL.Port()
	c.Username = proxyURL.User.Username()
	c.Password, _ = proxyURL.User.Password()

	return nil
}

type direct struct{}

// Direct is a direct proxy: one that makes network connections directly.
var Direct = direct{}

func (direct) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

// httpsDialer
type httpsDialer struct{}

// HTTPSDialer is a https proxy: one that makes network connections on tls.
var HttpsDialer = httpsDialer{}
var TlsConfig = &tls.Config{}

func (d httpsDialer) Dial(network, addr string) (c net.Conn, err error) {
	c, err = tls.Dial("tcp", addr, TlsConfig)
	if err != nil {
		fmt.Println(err)
	}
	return
}

// httpProxy is a HTTP/HTTPS connect proxy.
type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}

	return s, nil
}

func (s *httpProxy) Dial(network, addr string) (net.Conn, error) {
	// Dial and create the https client connection.
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	// HACK. http.ReadRequest also does this.
	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("User-Agent", "Powerby Gota")

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		// TODO close resp body ?
		resp.Body.Close()
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		c.Close()
		err = fmt.Errorf("Connect server using proxy error, StatusCode [%d]", resp.StatusCode)
		return nil, err
	}

	return c, nil
}

func FromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return proxy.FromURL(u, forward)
}

func FromEnvironment() proxy.Dialer {
	return proxy.FromEnvironment()
}

func NewHTTPDialer(config *Config) (proxy.Dialer, error) {
	if config == nil {
		return Direct, nil
	}

	rawURL := fmt.Sprintf("http://%s:%s@%s:%s", config.Username, config.Password, config.Host, config.Port)
	fmt.Println(rawURL)

	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	d, err := proxy.FromURL(proxyURL, Direct)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func init() {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)
}
