package articlemcp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// 管理页面可修改的博客域名仅可探测公网。检查实际拨号地址，覆盖 DNS 重绑定。
// 部署环境指定的博客继续使用部署方的网络配置，支持本地预览。
var publicBlogTransport = &http.Transport{
	DialContext: (&net.Dialer{Timeout: 4 * time.Second, ControlContext: func(_ context.Context, _, address string, _ syscall.RawConn) error {
		target, err := netip.ParseAddrPort(address)
		if err != nil || !publicBlogIP(target.Addr().Unmap()) {
			return errors.New("博客探测地址受限")
		}
		return nil
	}}).DialContext,
	IdleConnTimeout: 30 * time.Second,
}

var reservedBlogNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2001::/32"),
}

func publicBlogIP(ip netip.Addr) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || (ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip)) {
		return false
	}
	for _, reserved := range reservedBlogNetworks {
		if reserved.Contains(ip) {
			return false
		}
	}
	return true
}
