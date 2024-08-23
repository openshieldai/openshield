package lib

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/httprate"
	"github.com/openshieldai/openshield/models"
)

func GetModel(model string) (models.AiModels, error) {
	var aiModel = models.AiModels{Model: model}
	result := DB().Find(&aiModel)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.AiModels{}, result.Error
	}
	return aiModel, nil
}

func canonicalizeIP(ip string) string {
	isIPv6 := false
	// This is how net.ParseIP decides if an address is IPv6
	// https://cs.opensource.google/go/go/+/refs/tags/go1.17.7:src/net/ip.go;l=704
	for i := 0; !isIPv6 && i < len(ip); i++ {
		switch ip[i] {
		case '.':
			// IPv4
			return ip
		case ':':
			// IPv6
			isIPv6 = true
			break
		}
	}
	if !isIPv6 {
		// Not an IP address at all
		return ip
	}

	ipv6 := net.ParseIP(ip)
	if ipv6 == nil {
		return ip
	}

	return ipv6.Mask(net.CIDRMask(64, 128)).String()
}

func KeyByRealIP(r *http.Request) (string, error) {
	var ip string

	if tcip := r.Header.Get("True-Client-IP"); tcip != "" {
		ip = tcip
	} else if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip = xrip
	} else if cfrip := r.Header.Get("Cf-Connecting-IP"); cfrip != "" {
		ip = cfrip
	} else if cfripv6 := r.Header.Get("Cf-Connecting-IPv6"); cfripv6 != "" {
		ip = cfripv6
	} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		i := strings.Index(xff, ", ")
		if i == -1 {
			i = len(xff)
		}
		ip = xff[:i]
	} else {
		var err error
		ip, _, err = net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
	}

	return canonicalizeIP(ip), nil
}

type Option = httprate.Option

func WithKeyByRealIP() Option {
	return httprate.WithKeyFuncs(KeyByRealIP)
}
