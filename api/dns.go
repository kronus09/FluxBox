package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	DoHProviders = map[string]string{
		"aliyun":  "https://223.5.5.5/resolve",
		"tencent": "https://1.12.12.12/dns-query",
		"360":     "https://doh.360.cn/dns-query",
		"google":  "https://dns.google/dns-query",
	}

	globalSecureTransport *http.Transport
)

func GetHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: globalSecureTransport,
		Timeout:   timeout,
	}
}

func GetHTTPClientNoRedirect(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: globalSecureTransport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type DoHResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func getDoHServerURL() string {
	Mu.Lock()
	defer Mu.Unlock()

	if !MemoryGlobalConfig.NetworkEnableDoH {
		return ""
	}

	provider := MemoryGlobalConfig.NetworkDoHProvider
	if provider == "custom" {
		custom := strings.TrimSpace(MemoryGlobalConfig.NetworkDoHCustomURL)
		if custom != "" {
			return custom
		}
		return DoHProviders["aliyun"]
	}

	if url, ok := DoHProviders[provider]; ok {
		return url
	}
	return DoHProviders["aliyun"]
}

func LookupHostDoH(host string) ([]string, error) {
	serverURL := getDoHServerURL()
	if serverURL == "" {
		return nil, fmt.Errorf("DoH not enabled")
	}

	dnsQuery := buildDNSQuery(host)
	b64Query := base64.RawURLEncoding.EncodeToString(dnsQuery)

	url := fmt.Sprintf("%s?dns=%s", serverURL, b64Query)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/dns-message")

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DoH request failed: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	ips := parseDNSResponse(body)
	if len(ips) == 0 {
		ips, _ = lookupDoHJson(serverURL, host)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no DNS records found")
	}

	return ips, nil
}

func lookupDoHJson(serverURL, host string) ([]string, error) {
	url := fmt.Sprintf("%s?name=%s&type=A", serverURL, host)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/dns-json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DoHResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ips []string
	for _, ans := range result.Answer {
		if ans.Type == 1 && net.ParseIP(ans.Data) != nil {
			ips = append(ips, ans.Data)
		}
	}
	return ips, nil
}

func buildDNSQuery(host string) []byte {
	var buf bytes.Buffer

	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0x01, 0x00})
	buf.Write([]byte{0x00, 0x01})
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0x00, 0x00})
	buf.Write([]byte{0x00, 0x00})

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" {
			continue
		}
		buf.WriteByte(byte(len(label)))
		buf.WriteString(label)
	}
	buf.WriteByte(0x00)

	buf.Write([]byte{0x00, 0x01})
	buf.Write([]byte{0x00, 0x01})

	return buf.Bytes()
}

func parseDNSResponse(data []byte) []string {
	if len(data) < 12 {
		return nil
	}

	offset := 12
	for offset < len(data) {
		if data[offset] == 0 {
			offset++
			break
		}
		if data[offset] >= 192 {
			offset += 2
			break
		}
		offset += int(data[offset]) + 1
	}

	if offset+4 > len(data) {
		return nil
	}
	offset += 4

	ancount := int(data[6])<<8 | int(data[7])
	var ips []string

	for i := 0; i < ancount; i++ {
		for offset < len(data) {
			if data[offset] == 0 {
				offset++
				break
			}
			if data[offset] >= 192 {
				offset += 2
				break
			}
			offset += int(data[offset]) + 1
		}

		if offset+10 > len(data) {
			break
		}

		qType := int(data[offset])<<8 | int(data[offset+1])
		dataLen := int(data[offset+8])<<8 | int(data[offset+9])
		offset += 10

		if qType == 1 && dataLen == 4 && offset+4 <= len(data) {
			ip := fmt.Sprintf("%d.%d.%d.%d", data[offset], data[offset+1], data[offset+2], data[offset+3])
			ips = append(ips, ip)
		}
		offset += dataLen
	}

	return ips
}

func getSecureTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 10 * time.Second}

			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return dialer.DialContext(ctx, network, addr)
			}

			if net.ParseIP(host) != nil {
				return dialer.DialContext(ctx, network, addr)
			}

			ips, err := LookupHostDoH(host)
			if err == nil && len(ips) > 0 {
				for _, ip := range ips {
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
					if err == nil {
						return conn, nil
					}
				}
			}

			return dialer.DialContext(ctx, network, addr)
		},
	}
}

func InitSecureHTTPClient() {
	globalSecureTransport = getSecureTransport()
	http.DefaultClient = &http.Client{
		Transport: globalSecureTransport,
		Timeout:   30 * time.Second,
	}
}
