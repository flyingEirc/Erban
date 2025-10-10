package ssh

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

type Proxy struct {
	// URL of the proxy to use (leave empty for direct):
	// e.g. http://127.0.0.1:7890 or socks5://127.0.0.1:10808
	URL string
}

// dialThroughProxy dials targetAddr through the provided proxy URL.
// Supports socks5 and http proxies. For http, does a CONNECT handshake.
func dialThroughProxy(proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	switch strings.ToLower(proxyURL.Scheme) {
	case "socks5", "socks5h":
		var auth *xproxy.Auth
		if proxyURL.User != nil {
			user := proxyURL.User.Username()
			pass, _ := proxyURL.User.Password()
			auth = &xproxy.Auth{User: user, Password: pass}
		}
		dialer, err := xproxy.SOCKS5("tcp", hostPortOrDefault(proxyURL.Host, "1080"), auth, &net.Dialer{Timeout: 30 * time.Second})
		if err != nil {
			return nil, err
		}
		return dialer.Dial("tcp", targetAddr)
	case "http", "https":
		// Note: "https" here means TLS to proxy which isn't implemented; treat as plain CONNECT for simplicity.
		// Most local proxies accept plain http CONNECT.
		conn, err := net.DialTimeout("tcp", hostPortOrDefault(proxyURL.Host, "8080"), 30*time.Second)
		if err != nil {
			return nil, err
		}

		// Build CONNECT request
		var b strings.Builder
		b.WriteString(fmt.Sprintf("CONNECT %s HTTP/1.1\r\n", targetAddr))
		b.WriteString(fmt.Sprintf("Host: %s\r\n", targetAddr))
		if proxyURL.User != nil {
			user := proxyURL.User.Username()
			pass, _ := proxyURL.User.Password()
			token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
			b.WriteString("Proxy-Authorization: Basic ")
			b.WriteString(token)
			b.WriteString("\r\n")
		}
		b.WriteString("Connection: keep-alive\r\n\r\n")

		if _, err := conn.Write([]byte(b.String())); err != nil {
			_ = conn.Close()
			return nil, err
		}

		// Read minimal HTTP response
		br := bufio.NewReader(conn)
		line, err := br.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if !(strings.HasPrefix(line, "HTTP/1.1 200") || strings.HasPrefix(line, "HTTP/1.0 200")) {
			// Drain headers for error context (optional)
			for {
				h, e := br.ReadString('\n')
				if e != nil || h == "\r\n" || h == "\n" {
					break
				}
			}
			_ = conn.Close()
			return nil, errors.New("proxy CONNECT failed: " + strings.TrimSpace(line))
		}
		// Consume headers until empty line
		for {
			h, e := br.ReadString('\n')
			if e != nil || h == "\r\n" || h == "\n" {
				break
			}
		}
		// Now conn is a raw tunnel to targetAddr
		return conn, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}
}

func hostPortOrDefault(hostport, defPort string) string {
	if strings.Contains(hostport, ":") {
		return hostport
	}
	return net.JoinHostPort(hostport, defPort)
}
