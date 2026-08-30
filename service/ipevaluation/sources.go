//go:build !js

package ipevaluation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type (
	cloudPrefix struct {
		IPv4 string `json:"ip_prefix"`
		IPv6 string `json:"ipv6_prefix"`
	}

	googlePrefix struct {
		IPv4 string `json:"ipv4Prefix"`
		IPv6 string `json:"ipv6Prefix"`
	}

	azureDocument struct {
		Values []struct {
			Properties struct {
				Prefixes []string `json:"addressPrefixes"`
			} `json:"properties"`
		} `json:"values"`
	}

	threatFoxResponse struct {
		Data []struct {
			IOC     string `json:"ioc"`
			IOCType string `json:"ioc_type"`
		} `json:"data"`
	}
)

const maximumFeedBytes int64 = 256 * 1024 * 1024

var azureDownloadPattern *regexp.Regexp = regexp.MustCompile(`https://download\.microsoft\.com/download/[^"']+ServiceTags_Public_[0-9]+\.json`)

func download(ctx context.Context, client *http.Client, method, url string, body []byte, headers map[string]string) (contents []byte, err error) {
	var request *http.Request
	if request, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body)); err != nil {
		return
	}

	request.Header.Set("User-Agent", "Sigil-IP-Evaluation/1.0")
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	var response *http.Response
	if response, err = client.Do(request); err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err = fmt.Errorf("%s returned HTTP %d", url, response.StatusCode)
		return
	}

	if response.ContentLength > maximumFeedBytes {
		err = fmt.Errorf("%s exceeds the feed size limit", url)
		return
	}

	if contents, err = io.ReadAll(io.LimitReader(response.Body, maximumFeedBytes+1)); err != nil {
		return
	}

	if int64(len(contents)) > maximumFeedBytes {
		err = fmt.Errorf("%s exceeds the feed size limit", url)
	}
	return
}

func prefixFromValue(value string) (prefix netip.Prefix, valid bool) {
	var (
		address     netip.Addr
		addressPort netip.AddrPort
		fields      []string
		parsedURL   *url.URL
		err         error
	)

	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") {
		return
	}

	if fields = strings.Fields(value); len(fields) > 0 {
		value = strings.TrimSpace(fields[0])
	}

	if strings.Contains(value, "://") {
		if parsedURL, err = url.Parse(value); err != nil {
			return
		}
		value = parsedURL.Host
	}

	if strings.Contains(value, ":") && !strings.Contains(value, "/") {
		if addressPort, err = netip.ParseAddrPort(value); err == nil {
			prefix = netip.PrefixFrom(addressPort.Addr().Unmap(), addressPort.Addr().Unmap().BitLen())
			valid = true
			return
		}
	}

	if address, err = netip.ParseAddr(value); err == nil {
		address = address.Unmap()
		prefix = netip.PrefixFrom(address, address.BitLen())
		valid = true
		return
	}

	if prefix, err = netip.ParsePrefix(value); err == nil {
		prefix = prefix.Masked()
		valid = prefix.Addr().IsGlobalUnicast() && !prefix.Addr().IsPrivate()
	}
	return
}

func torFeed(client *http.Client) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		var contents []byte
		if contents, err = download(ctx, client, http.MethodGet, "https://onionoo.torproject.org/details?type=relay&flag=Exit&fields=exit_addresses", nil, nil); err != nil {
			return
		}

		var document struct {
			Relays []struct {
				ExitAddresses []string `json:"exit_addresses"`
			} `json:"relays"`
		}
		if err = json.Unmarshal(contents, &document); err != nil {
			return
		}

		for _, relay := range document.Relays {
			for _, address := range relay.ExitAddresses {
				var (
					prefix netip.Prefix
					valid  bool
				)
				if prefix, valid = prefixFromValue(address); valid {
					indicators = append(indicators, Indicator{Prefix: prefix, Category: CategoryTor, Confidence: 1})
				}
			}
		}

		indicators = normalize(indicators)
		return
	}
}

func normalize(indicators []Indicator) (normalized []Indicator) {
	var unique map[string]Indicator = make(map[string]Indicator, len(indicators))
	for _, indicator := range indicators {
		indicator.Prefix = indicator.Prefix.Masked()
		if !indicator.Prefix.IsValid() || !indicator.Prefix.Addr().IsGlobalUnicast() || indicator.Prefix.Addr().IsPrivate() {
			continue
		}

		unique[string(indicator.Category)+"\x00"+indicator.Prefix.String()] = indicator
	}

	normalized = make([]Indicator, 0, len(unique))
	for _, indicator := range unique {
		normalized = append(normalized, indicator)
	}
	return
}

func textFeed(client *http.Client, url string, category Category, confidence float64) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		var contents []byte
		if contents, err = download(ctx, client, http.MethodGet, url, nil, nil); err != nil {
			return
		}

		var scanner *bufio.Scanner = bufio.NewScanner(bytes.NewReader(contents))
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			var (
				prefix netip.Prefix
				valid  bool
			)
			if prefix, valid = prefixFromValue(scanner.Text()); valid {
				indicators = append(indicators, Indicator{Prefix: prefix, Category: category, Confidence: confidence})
			}
		}

		if err = scanner.Err(); err == nil {
			indicators = normalize(indicators)
		}
		return
	}
}

func fallbackFeed(primary, fallback feedFetcher) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		if indicators, err = primary(ctx); err != nil {
			indicators, err = fallback(ctx)
		}
		return
	}
}

func cloudJSONFeed(client *http.Client, url string, google bool) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		var contents []byte
		if contents, err = download(ctx, client, http.MethodGet, url, nil, nil); err != nil {
			return
		}

		if google {
			var document struct {
				Prefixes []googlePrefix `json:"prefixes"`
			}
			if err = json.Unmarshal(contents, &document); err != nil {
				return
			}
			for _, item := range document.Prefixes {
				for _, value := range []string{item.IPv4, item.IPv6} {
					var (
						prefix netip.Prefix
						valid  bool
					)
					if prefix, valid = prefixFromValue(value); valid {
						indicators = append(indicators, Indicator{Prefix: prefix, Category: CategoryHosting, Confidence: 1})
					}
				}
			}
		} else {
			var document struct {
				Prefixes     []cloudPrefix `json:"prefixes"`
				IPv6Prefixes []cloudPrefix `json:"ipv6_prefixes"`
			}
			if err = json.Unmarshal(contents, &document); err != nil {
				return
			}
			for _, item := range append(document.Prefixes, document.IPv6Prefixes...) {
				for _, value := range []string{item.IPv4, item.IPv6} {
					var (
						prefix netip.Prefix
						valid  bool
					)
					if prefix, valid = prefixFromValue(value); valid {
						indicators = append(indicators, Indicator{Prefix: prefix, Category: CategoryHosting, Confidence: 1})
					}
				}
			}
		}

		indicators = normalize(indicators)
		return
	}
}

func azureFeed(client *http.Client) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		var page []byte
		if page, err = download(ctx, client, http.MethodGet, "https://www.microsoft.com/download/details.aspx?id=56519", nil, nil); err != nil {
			return
		}

		var location string = azureDownloadPattern.FindString(string(page))
		if location == "" {
			err = fmt.Errorf("Azure service-tag download URL was not found")
			return
		}

		var contents []byte
		if contents, err = download(ctx, client, http.MethodGet, location, nil, nil); err != nil {
			return
		}

		var document azureDocument
		if err = json.Unmarshal(contents, &document); err != nil {
			return
		}

		for _, value := range document.Values {
			for _, address := range value.Properties.Prefixes {
				var (
					prefix netip.Prefix
					valid  bool
				)
				if prefix, valid = prefixFromValue(address); valid {
					indicators = append(indicators, Indicator{Prefix: prefix, Category: CategoryHosting, Confidence: 1})
				}
			}
		}

		indicators = normalize(indicators)
		return
	}
}

func threatFoxFeed(client *http.Client, authKey string) feedFetcher {
	return func(ctx context.Context) (indicators []Indicator, err error) {
		var contents []byte
		if contents, err = download(ctx, client, http.MethodPost, "https://threatfox-api.abuse.ch/api/v1/",
			[]byte(`{"query":"get_iocs","days":1}`), map[string]string{"Auth-Key": authKey, "Content-Type": "application/json"}); err != nil {
			return
		}

		var response threatFoxResponse
		if err = json.Unmarshal(contents, &response); err != nil {
			return
		}

		for _, item := range response.Data {
			if !strings.Contains(item.IOCType, "ip") {
				continue
			}
			var (
				prefix netip.Prefix
				valid  bool
			)
			if prefix, valid = prefixFromValue(item.IOC); valid {
				indicators = append(indicators, Indicator{Prefix: prefix, Category: CategoryMalicious, Confidence: 0.9})
			}
		}

		indicators = normalize(indicators)
		return
	}
}

func defaultSources(authKey string) (sources []feedSource) {
	var client *http.Client = &http.Client{Timeout: 2 * time.Minute}
	sources = []feedSource{
		{name: "proxyscrape", interval: 15 * time.Minute, ttl: 24 * time.Hour, fetch: textFeed(client, "https://raw.githubusercontent.com/ProxyScrape/free-proxy-list/refs/heads/main/proxies/all/data.txt", CategoryOpenProxy, 0.65)},
		{name: "tor", interval: time.Hour, ttl: 6 * time.Hour, fetch: fallbackFeed(torFeed(client), textFeed(client, "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/tor_exits.ipset", CategoryTor, 0.9))},
		{name: "aws", interval: 12 * time.Hour, ttl: 48 * time.Hour, fetch: cloudJSONFeed(client, "https://ip-ranges.amazonaws.com/ip-ranges.json", false)},
		{name: "google-cloud", interval: 12 * time.Hour, ttl: 48 * time.Hour, fetch: cloudJSONFeed(client, "https://www.gstatic.com/ipranges/cloud.json", true)},
		{name: "azure", interval: 12 * time.Hour, ttl: 48 * time.Hour, fetch: azureFeed(client)},
		{name: "cloudflare-v4", interval: 12 * time.Hour, ttl: 48 * time.Hour, fetch: textFeed(client, "https://www.cloudflare.com/ips-v4/", CategoryHosting, 1)},
		{name: "cloudflare-v6", interval: 12 * time.Hour, ttl: 48 * time.Hour, fetch: textFeed(client, "https://www.cloudflare.com/ips-v6/", CategoryHosting, 1)},
		{name: "firehol-proxies", interval: 24 * time.Hour, ttl: 48 * time.Hour, fetch: textFeed(client, "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/firehol_proxies.netset", CategoryOpenProxy, 0.6)},
	}

	if authKey != "" {
		sources = append(sources, feedSource{name: "threatfox", interval: time.Hour, ttl: 7 * 24 * time.Hour, fetch: threatFoxFeed(client, authKey)})
	}
	return
}
