package control

import (
	"encoding/base64"
	"errors"
	"fmt"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net"
	"slices"
	"strings"
)

func validateProfile(p c.Profile) error {
	if len(p.Processes)+len(p.Paths) == 0 {
		return errors.New("at least one game process or full executable path is required")
	}
	if len(p.Processes)+len(p.Paths)+len(p.Domains)+len(p.CIDRs) > 200 {
		return errors.New("profile is too large")
	}
	for _, s := range append(append(append([]string{}, p.Processes...), p.Paths...), append(p.Domains, p.ExcludeDomains...)...) {
		if strings.TrimSpace(s) == "" || strings.ContainsAny(s, ",\n\r\x00") {
			return errors.New("invalid rule value")
		}
	}
	for _, cidr := range p.CIDRs {
		if _, _, e := net.ParseCIDR(cidr); e != nil {
			return errors.New("invalid CIDR")
		}
	}
	return nil
}
func validHost(s string) bool {
	return s != "" && len(s) < 254 && !strings.ContainsAny(s, " /\\\r\n\x00,\"'")
}
func validKey(method, key string) bool {
	n := 16
	if method == "2022-blake3-aes-256-gcm" {
		n = 32
	} else if method != "2022-blake3-aes-128-gcm" {
		return false
	}
	b, e := base64.StdEncoding.DecodeString(key)
	return e == nil && len(b) == n
}
func (s *Server) compile(tx *gorm.DB, l Line) ([]c.Hop, error) {
	if len(l.Regions) == 0 || len(l.EndpointIDs) == 0 || len(l.EndpointIDs) > 5 {
		return nil, errors.New("line requires regions and one to five endpoints")
	}
	if l.Capacity < 1 || l.Capacity > 10000 {
		return nil, errors.New("capacity must be between 1 and 10000")
	}
	if l.Mode != "managed" && l.Mode != "imported" && l.Mode != "hybrid" {
		return nil, errors.New("mode must be managed, imported or hybrid")
	}
	if l.Mode != "managed" && !s.Config.AllowImported {
		return nil, errors.New("imported lines require ALLOW_IMPORTED=true; external credentials cannot be revoked by this platform")
	}
	if l.Mode == "imported" && len(l.EndpointIDs) != 1 {
		return nil, errors.New("imported lines use one preconfigured SS2022 ingress endpoint")
	}
	seen := map[string]bool{}
	hops := []c.Hop{}
	for i, id := range l.EndpointIDs {
		if seen[id] {
			return nil, errors.New("cyclic or duplicated endpoint")
		}
		seen[id] = true
		var e Endpoint
		if tx.First(&e, "id = ?", id).Error != nil || !e.Enabled {
			return nil, errors.New("endpoint unavailable")
		}
		minimumPort := 1
		if e.Capabilities.Managed {
			minimumPort = 1024
		}
		if !validHost(e.Host) || e.Port < minimumPort || e.PortEnd < e.Port || e.PortEnd > 65535 {
			return nil, errors.New("invalid endpoint address or port pool")
		}
		var n Node
		if tx.First(&n, "id = ?", e.NodeID).Error != nil || !n.Enabled {
			return nil, errors.New("node unavailable")
		}
		if !e.Capabilities.TCP || !n.Capabilities.TCP {
			return nil, errors.New("every hop must support TCP")
		}
		if e.Capabilities.UDP && !n.Capabilities.UDP {
			return nil, errors.New("endpoint UDP declaration exceeds node capabilities")
		}
		if e.Capabilities.IPv6 && !n.Capabilities.IPv6 {
			return nil, errors.New("node does not support IPv6")
		}
		if net.ParseIP(e.Host) != nil && strings.Contains(e.Host, ":") && !e.Capabilities.IPv6 {
			return nil, errors.New("IPv6 ingress requires IPv6 capability")
		}
		if i == len(l.EndpointIDs)-1 {
			if e.Kind != "ss2022" {
				return nil, errors.New("exit must terminate SS2022")
			}
			if e.Cipher != "2022-blake3-aes-128-gcm" && e.Cipher != "2022-blake3-aes-256-gcm" {
				return nil, errors.New("unsupported SS2022 cipher")
			}
		} else if e.Kind != "realm" {
			return nil, errors.New("intermediate hops must be transparent Realm forwarding endpoints")
		}
		if (l.Mode == "managed" || (l.Mode == "hybrid" && i < len(l.EndpointIDs)-1)) && (!n.Capabilities.Managed || !e.Capabilities.Managed || timeSince(n.LastSeen) > s.Config.NodeTimeout) {
			return nil, errors.New("managed node must be online and support leased runtimes")
		}
		if (l.Mode == "imported" || (l.Mode == "hybrid" && i == len(l.EndpointIDs)-1)) && e.CredentialID == "" {
			return nil, errors.New("imported endpoint requires protocol credentials")
		}
		if e.OwnerID != "" {
			if len(l.UserIDs) != 1 || l.UserIDs[0] != e.OwnerID {
				return nil, errors.New("private egress requires an owner-only line policy")
			}
		}
		if l.Mode == "hybrid" && i == len(l.EndpointIDs)-1 && e.Capabilities.Managed {
			return nil, errors.New("hybrid exit must be externally managed")
		}
		hops = append(hops, c.Hop{EndpointID: e.ID, NodeID: n.ID, Kind: e.Kind, Host: e.Host, Port: e.Port, PortEnd: e.PortEnd, Cipher: e.Cipher, CredentialID: e.CredentialID, Capabilities: e.Capabilities})
	}
	for _, gameID := range l.GameIDs {
		var game Game
		if tx.First(&game, "id = ?", gameID).Error != nil {
			return nil, errors.New("unknown game restriction")
		}
		if game.Draft.UDP {
			for _, h := range hops {
				if !h.Capabilities.UDP {
					return nil, fmt.Errorf("game %s requires UDP across the complete path", gameID)
				}
			}
		}
	}
	return hops, nil
}
func permitted(l Line, u User) bool {
	return len(l.UserIDs) == 0 || slices.Contains(l.UserIDs, u.ID) || u.Role == "admin"
}
func liveState(state string) bool { return state != "CLOSED" }

func egressAllowed(tx *gorm.DB, endpointID, userID string) bool {
	var endpoint Endpoint
	if tx.First(&endpoint, "id = ?", endpointID).Error != nil || !endpoint.Enabled {
		return false
	}
	if endpoint.OwnerID != "" && endpoint.OwnerID != userID {
		return false
	}
	var count int64
	tx.Model(&Binding{}).Where("endpoint_id = ? AND enabled = ? AND kind = ? AND user_id <> ?", endpointID, true, "dedicated", userID).Count(&count)
	return count == 0
}
