// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netx

import (
	"errors"
	"fmt"

	"github.com/miekg/dns"
)

var ErrDNSQueryFailed = errors.New("DNS query failed")

func DNSQuery(server, target string) error {
	config := dns.ClientConfig{Servers: []string{server}, Port: "53"}

	client := dns.Client{}

	message := dns.Msg{}

	message.SetQuestion(target, dns.TypeA)

	r, _, err := client.Exchange(&message, config.Servers[0]+":"+config.Port)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDNSQueryFailed, err)
	}

	if r.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("%w: %s", ErrDNSQueryFailed, dns.RcodeToString[r.Rcode])
	}

	return nil
}
