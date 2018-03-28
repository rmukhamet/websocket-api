// discover
package consul

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

var ConsulURL = "172.22.2.2"

func DiscoverNATS() string {
	const (
		natsService    = "nats.service.consul."
		natsClientPort = "4222"
		consulDNSPort  = "8600"
	)

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(natsService), dns.TypeA)

	r, _, err := c.Exchange(m, net.JoinHostPort(ConsulURL, consulDNSPort))
	if r == nil {
		log.Fatalln("Can't resolve NATS ip-address: ", err)
	}
	if r.Rcode != dns.RcodeSuccess {
		log.Fatalf("Invalid answer name %s after A query for %s\n", natsService, natsService)
	}

	for _, a := range r.Answer {
		if a.Header().Rrtype == dns.TypeA {
			return fmt.Sprintf("nats://%s:%s", a.(*dns.A).A, natsClientPort)
		}
	}

	log.Fatalln("Can't resolve NATS ip-address from Consul DNS Interface")
	return ""
}
