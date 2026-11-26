package intercom

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

/*
inventoryFiles notes how each attachment is served: its host, size,
whether its link is signed and when it expires, and whether it downloads
without the token. It reads at most a few bytes.
*/
func (s *Source) inventoryFiles(ctx context.Context, r *Report, list []attachment) {
	for _, a := range list {
		u, err := url.Parse(a.URL)
		if err != nil || u.Host == "" {
			continue
		}

		r.Attachments.Count++
		r.Attachments.Hosts[u.Host]++
		r.Attachments.LargestBytes = max(r.Attachments.LargestBytes, a.Filesize)

		noteSigned(&r.Attachments, u.RawQuery)
		probeFile(ctx, &r.Attachments, a)
	}
}

/*
noteSigned notes the signature and expiry parameters of a link's query.
*/
func noteSigned(r *AttachmentReport, query string) {
	m := expiring.FindAllStringSubmatch(query, -1)
	if len(m) == 0 {
		return
	}

	r.SignedLinks++

	for _, p := range m {
		name := strings.ToLower(p[1])
		if !contains(r.ExpiryParams, name) {
			r.ExpiryParams = append(r.ExpiryParams, name)
		}

		if name != "expires" {
			continue
		}

		if at, err := strconv.ParseInt(p[2], 10, 64); err == nil {
			r.ExpiresInSeconds = append(r.ExpiresInSeconds, at-time.Now().Unix())
		}
	}
}

/*
probeFile asks for an attachment's first bytes without the token, and
reads its size from the answer when Intercom gave none.
*/
func probeFile(ctx context.Context, r *AttachmentReport, a attachment) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, http.NoBody)
	if err != nil {
		return
	}

	req.Header.Set("Range", "bytes=0-15")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64))
	_ = res.Body.Close()

	if res.StatusCode >= 300 {
		r.RefusedNoAuth++

		return
	}

	r.DownloadedNoAuth++

	if a.Filesize != 0 {
		return
	}

	if n, ok := rangeSize(res.Header.Get("Content-Range")); ok {
		r.LargestBytes = max(r.LargestBytes, n)
	}
}

/*
rangeSize reads the full size from a Content-Range header, bytes 0-15/size.
*/
func rangeSize(cr string) (int64, bool) {
	i := strings.LastIndex(cr, "/")
	if cr == "" || i < 0 {
		return 0, false
	}

	n, err := strconv.ParseInt(cr[i+1:], 10, 64)

	return n, err == nil
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}
