package socks

import (
	"context"
	"net"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/v2fly/v2ray-core/v5/common/net/proxy/socks5"
	"github.com/miekg/dns"
)

type SOCKSPlugin struct {
	Next     plugin.Handler
	Hostname string
	Port     string
	DNSServer string
}

func (s *SOCKSPlugin) Name() string { return "socks" }

func (s *SOCKSPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	// SOCKSプロキシの設定
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return socks5.Dial(context.Background(), &socks5.DialerConfig{
			Address: &net.TCPAddr{
				IP:   net.ParseIP(s.Hostname),
				Port: portToInt(s.Port),
			},
		})
	}

	// DNS転送
	c := &dns.Client{
		Dialer: dialFunc,
	}

	// 指定されたDNSサーバーに転送
	resp, _, err := c.Exchange(r, net.JoinHostPort(s.DNSServer, "53"))
	if err != nil {
		return plugin.NextOrFailure(s.Name(), s.Next, ctx, w, r)
	}

	// レスポンスを書き戻す
	w.WriteMsg(resp)
	return dns.RcodeSuccess, nil
}

// ポート文字列を整数に変換するヘルパー関数
func portToInt(port string) int {
	p, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return p
}

// プラグインのセットアップ関数
func Setup(next plugin.Handler) func(c *dnsserver.Config) error {
	return func(c *dnsserver.Config) error {
		c.AddPlugin(func(next plugin.Handler) plugin.Handler {
			return &SOCKSPlugin{
				Next: next,
				// デフォルト値や設定からの読み込みを追加
			}
		})
		return nil
	}
}
