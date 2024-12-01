package socks

import (
	"context"
	"net"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"

	"github.com/cretz/go-socks5"
	"github.com/miekg/dns"
)

type SOCKSPlugin struct {
	Next     plugin.Handler
	Domains  []string
	SOCKSConfig *socks5.Config
}

func (s *SOCKSPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	// 対象ドメインかどうかをチェック
	if s.shouldProxyDomain(qname) {
		return s.proxyDNSRequest(ctx, w, r)
	}

	// 通常のDNS解決
	return plugin.NextOrFailure(s.Name(), s.Next, ctx, w, r)
}

func (s *SOCKSPlugin) shouldProxyDomain(domain string) bool {
	for _, d := range s.Domains {
		if dns.IsSubDomain(d, domain) {
			return true
		}
	}
	return false
}

func (s *SOCKSPlugin) proxyDNSRequest(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// SOCKS5プロキシサーバーを作成
	server, err := socks5.New(s.SOCKSConfig)
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	// DNS over SOCKSプロキシ
	dialer, err := server.Dial("tcp", "8.8.8.8:53")
	if err != nil {
		return dns.RcodeServerFailure, err
	}
	defer dialer.Close()

	// DNSクエリを送信
	dnsConn := &dns.Conn{Conn: dialer}
	if err := dnsConn.WriteMsg(r); err != nil {
		return dns.RcodeServerFailure, err
	}

	// 応答を受信
	resp, err := dnsConn.ReadMsg()
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	// 応答を返す
	if err := w.WriteMsg(resp); err != nil {
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeSuccess, nil
}

func (s *SOCKSPlugin) Name() string { return "socks" }

func init() {
	plugin.Register("socks", setup)
}

func setup(c *dnsserver.Controller) error {
	// プラグインの設定をパース
	socks := &SOCKSPlugin{}
	
	for c.Next() {
		// 設定ブロックからドメインとSOCKS設定を読み取る
		args := c.RemainingArgs()
		if len(args) < 2 {
			return c.ArgErr()
		}

		socks.Domains = args[1:]
		socks.SOCKSConfig = &socks5.Config{
			ProxyAddr: args[0],
			// 必要に応じて認証情報を追加
		}
	}

	c.AddPlugin(func(next plugin.Handler) plugin.Handler {
		socks.Next = next
		return socks
	})

	return nil
}
