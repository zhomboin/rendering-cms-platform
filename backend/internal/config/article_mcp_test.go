package config

import "testing"

func TestArticleMCPConfiguration(t *testing.T) {
	for _, key := range []string{"MCP_ORIGIN", "MCP_ADMIN_ORIGIN", "MCP_BLOG_ORIGIN", "MCP_OAUTH_ISSUER", "MCP_OAUTH_PUBLIC_KEY"} {
		t.Setenv(key, "")
	}
	c, err := loadArticleMCP(false)
	if err != nil || c.Origin != "" {
		t.Fatal(c, err)
	}
	t.Setenv("MCP_ORIGIN", "http://127.0.0.1")
	if _, err = loadArticleMCP(true); err == nil {
		t.Fatal("缺少博客来源")
	}
	t.Setenv("MCP_BLOG_ORIGIN", "https://blog.example.invalid")
	c, err = loadArticleMCP(true)
	if err != nil || c.AdminOrigin != c.Origin {
		t.Fatal(c, err)
	}
	if _, err = loadArticleMCP(false); err == nil {
		t.Fatal("生产接受明文 HTTP")
	}
	t.Setenv("MCP_ORIGIN", "https://cms.example.invalid")
	t.Setenv("MCP_OAUTH_ISSUER", "https://identity.example.invalid")
	if _, err = loadArticleMCP(false); err == nil {
		t.Fatal("OAuth 缺少验签公钥")
	}
	for _, bad := range []string{"https://user@blog.example.invalid", "https://blog.example.invalid/path", "http://blog.example.invalid", "https://blog.example.invalid/?q=1"} {
		if _, err = mcpOrigin(bad, true); err == nil {
			t.Fatalf("应拒绝 %s", bad)
		}
	}
}
