package dns_test

import (
	"errors"
	"net"
	"strings"
	"testing"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/dimmihq/dimmi/framework/dns"
)

/*
serveZone answers DNS queries over UDP from the zone, the way an
authoritative server would, and returns its address.
*/
func serveZone(t *testing.T, zone map[string]dns.Records) string {
	t.Helper()

	var lc net.ListenConfig

	conn, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}

			if reply, err := respond(zone, buf[:n]); err == nil {
				_, _ = conn.WriteTo(reply, addr)
			}
		}
	}()

	return conn.LocalAddr().String()
}

/*
silentServer accepts DNS queries and never answers them.
*/
func silentServer(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	conn, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return conn.LocalAddr().String()
}

func respond(zone map[string]dns.Records, query []byte) ([]byte, error) {
	var p dnsmessage.Parser

	header, err := p.Start(query)
	if err != nil {
		return nil, err
	}

	question, err := p.Question()
	if err != nil {
		return nil, err
	}

	header.Response, header.Authoritative, header.RecursionAvailable = true, true, true
	b := dnsmessage.NewBuilder(nil, header)
	b.EnableCompression()

	name := strings.ToLower(strings.TrimSuffix(question.Name.String(), "."))
	if _, ok := zone[name]; !ok {
		header.RCode = dnsmessage.RCodeNameError
		b = dnsmessage.NewBuilder(nil, header)
	}

	if err := b.StartQuestions(); err != nil {
		return nil, err
	}

	if err := b.Question(question); err != nil {
		return nil, err
	}

	if err := b.StartAnswers(); err != nil {
		return nil, err
	}

	if err := answer(&b, zone, name, question.Type); err != nil {
		return nil, err
	}

	return b.Finish()
}

/*
answer writes the records of the name and type, writing each alias on
the way as a CNAME record first.
*/
func answer(b *dnsmessage.Builder, zone map[string]dns.Records, name string, qtype dnsmessage.Type) error {
	records, ok := zone[name]
	if !ok {
		return nil
	}

	if records.CNAME != "" {
		target := dnsmessage.MustNewName(records.CNAME + ".")
		if err := b.CNAMEResource(rrHeader(name), dnsmessage.CNAMEResource{CNAME: target}); err != nil {
			return err
		}

		if qtype == dnsmessage.TypeCNAME {
			return nil
		}

		return answer(b, zone, records.CNAME, qtype)
	}

	return writeRecords(b, name, records, qtype)
}

func writeRecords(b *dnsmessage.Builder, name string, records dns.Records, qtype dnsmessage.Type) error {
	var errs []error

	for _, addr := range records.Addrs {
		switch {
		case qtype == dnsmessage.TypeA && addr.Is4():
			errs = append(errs, b.AResource(rrHeader(name), dnsmessage.AResource{A: addr.As4()}))
		case qtype == dnsmessage.TypeAAAA && addr.Is6():
			errs = append(errs, b.AAAAResource(rrHeader(name), dnsmessage.AAAAResource{AAAA: addr.As16()}))
		}
	}

	for _, mx := range records.MX {
		if qtype == dnsmessage.TypeMX {
			mxr := dnsmessage.MXResource{Pref: mx.Pref, MX: dnsmessage.MustNewName(mx.Host + ".")}
			errs = append(errs, b.MXResource(rrHeader(name), mxr))
		}
	}

	for _, host := range records.NS {
		if qtype == dnsmessage.TypeNS {
			errs = append(errs, b.NSResource(rrHeader(name), dnsmessage.NSResource{NS: dnsmessage.MustNewName(host + ".")}))
		}
	}

	for _, txt := range records.TXT {
		if qtype == dnsmessage.TypeTXT {
			errs = append(errs, b.TXTResource(rrHeader(name), dnsmessage.TXTResource{TXT: chunks(txt)}))
		}
	}

	return errors.Join(errs...)
}

func rrHeader(name string) dnsmessage.ResourceHeader {
	return dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(name + "."), Class: dnsmessage.ClassINET, TTL: 60}
}

/*
chunks splits a TXT value into the 255-byte strings DNS can carry.
*/
func chunks(value string) []string {
	var parts []string
	for len(value) > 255 {
		parts = append(parts, value[:255])
		value = value[255:]
	}

	return append(parts, value)
}
