package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

type ArticleMCPConfig struct {
	Origin, AdminOrigin, BlogOrigin string
	Issuer                          string
	Key                             *rsa.PublicKey
	Development                     bool
}

func mcpOrigin(raw string, dev bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Trim(u.EscapedPath(), "/") != "" {
		return "", errors.New("MCP origin must be a bare origin")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || ip.IsLoopback()
	if u.Scheme != "https" && !(dev && local && u.Scheme == "http") {
		return "", errors.New("MCP origin requires HTTPS or a development loopback address")
	}
	return u.Scheme + "://" + u.Host, nil
}
func loadArticleMCP(dev bool) (ArticleMCPConfig, error) {
	c := ArticleMCPConfig{Development: dev}
	origin, admin, blog := os.Getenv("MCP_ORIGIN"), os.Getenv("MCP_ADMIN_ORIGIN"), os.Getenv("MCP_BLOG_ORIGIN")
	issuer, key := os.Getenv("MCP_OAUTH_ISSUER"), os.Getenv("MCP_OAUTH_PUBLIC_KEY")
	if origin == "" && admin == "" && blog == "" && issuer == "" && key == "" {
		return c, nil
	}
	if origin != "" || admin != "" || blog != "" {
		if err := c.SetOrigins(origin, admin, blog); err != nil {
			return c, err
		}
	}
	if issuer == "" && key == "" {
		return c, nil
	}
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return c, errors.New("MCP_OAUTH_ISSUER requires an HTTPS issuer URL")
	}
	der, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return c, errors.New("invalid MCP OAuth public key")
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return c, errors.New("invalid MCP OAuth SPKI public key")
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok || rsaKey.N.BitLen() < 2048 {
		return c, errors.New("MCP OAuth requires an RSA public key of at least 2048 bits")
	}
	c.Issuer, c.Key = issuer, rsaKey
	return c, nil
}

func (c *ArticleMCPConfig) SetOrigins(origin, admin, blog string) error {
	if admin == "" {
		admin = origin
	}
	var err error
	if c.Origin, err = mcpOrigin(origin, c.Development); err != nil {
		return errors.New("请填写有效的 CMS API 来源")
	}
	if c.AdminOrigin, err = mcpOrigin(admin, c.Development); err != nil {
		return errors.New("请填写有效的后台来源")
	}
	if c.BlogOrigin, err = mcpOrigin(blog, c.Development); err != nil {
		return errors.New("请填写有效的博客来源")
	}
	return nil
}
