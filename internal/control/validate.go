package control

import (
	"encoding/base64"
	c "github.com/mbfczzz/FlowLight-Accelerator/packages/contracts"
	"gorm.io/gorm"
	"net"
	"slices"
	"strings"
)

func validateProfile(p c.Profile) error {
	if len(p.Processes)+len(p.Paths) == 0 {
		return badRequest("请至少填写一个游戏进程名或完整程序路径")
	}
	if len(p.Processes)+len(p.Paths)+len(p.Domains)+len(p.CIDRs) > 200 {
		return badRequest("规则条目过多（最多 200 条），请精简后再保存")
	}
	for _, s := range append(append(append([]string{}, p.Processes...), p.Paths...), append(p.Domains, p.ExcludeDomains...)...) {
		if strings.TrimSpace(s) == "" || strings.ContainsAny(s, ",\n\r\x00") {
			return badRequest("规则中有空行，或包含逗号、换行等不允许的字符")
		}
	}
	for _, cidr := range p.CIDRs {
		if _, _, e := net.ParseCIDR(cidr); e != nil {
			return badRequest("目标网段格式不正确，例如 203.0.113.0/24")
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
		return nil, badRequest("线路需要至少一个区服，以及 1–5 个服务端点")
	}
	if l.Capacity < 1 || l.Capacity > 10000 {
		return nil, badRequest("并发容量须在 1–10000 之间")
	}
	if l.Mode != "managed" && l.Mode != "imported" && l.Mode != "hybrid" {
		return nil, badRequest("运行模式只能是平台托管、导入现成线路或平台入口＋自带落地")
	}
	if l.Mode != "managed" && !s.Config.AllowImported {
		return nil, badRequest("导入线路需要在控制服务配置 ALLOW_IMPORTED=true；外部凭据无法由本平台撤销")
	}
	if l.Mode == "imported" && len(l.EndpointIDs) != 1 {
		return nil, badRequest("导入线路只能使用一个预先配置好的 SS2022 入口端点")
	}
	seen := map[string]bool{}
	hops := []c.Hop{}
	for i, id := range l.EndpointIDs {
		if seen[id] {
			return nil, badRequest("线路中有重复的服务端点")
		}
		seen[id] = true
		var e Endpoint
		if tx.First(&e, "id = ?", id).Error != nil || !e.Enabled {
			return nil, badRequest("线路中的服务端点不存在或已停用")
		}
		minimumPort := 1
		if e.Capabilities.Managed {
			minimumPort = 1024
		}
		if !validHost(e.Host) || e.Port < minimumPort || e.PortEnd < e.Port || e.PortEnd > 65535 {
			return nil, badRequest("服务端点的地址或端口范围不正确")
		}
		var n Node
		if tx.First(&n, "id = ?", e.NodeID).Error != nil || !n.Enabled {
			return nil, badRequest("端点所在的节点服务器不存在或已停用")
		}
		if !e.Capabilities.TCP || !n.effective().TCP {
			return nil, badRequest("线路的每一跳都必须支持 TCP")
		}
		if e.Capabilities.UDP && !n.effective().UDP {
			return nil, badRequest("端点声明支持 UDP，但所在的节点服务器不支持")
		}
		if e.Capabilities.IPv6 && !n.effective().IPv6 {
			return nil, badRequest("端点声明支持 IPv6，但所在的节点服务器不支持")
		}
		if net.ParseIP(e.Host) != nil && strings.Contains(e.Host, ":") && !e.Capabilities.IPv6 {
			return nil, badRequest("入口地址是 IPv6，但端点未声明支持 IPv6")
		}
		if i == len(l.EndpointIDs)-1 {
			if e.Kind != "ss2022" {
				return nil, badRequest("线路最后一跳必须是 SS2022 落地端点")
			}
			if e.Cipher != "2022-blake3-aes-128-gcm" && e.Cipher != "2022-blake3-aes-256-gcm" {
				return nil, badRequest("不支持的 SS2022 加密方法")
			}
		} else if e.Kind != "realm" {
			return nil, badRequest("入口和中转必须是 Realm 透明中转端点")
		}
		if (l.Mode == "managed" || (l.Mode == "hybrid" && i < len(l.EndpointIDs)-1)) && (!n.effective().Managed || !e.Capabilities.Managed || timeSince(n.LastSeen) > s.Config.NodeTimeout) {
			return nil, badRequest("平台托管的节点必须在线并支持托管进程")
		}
		if (l.Mode == "imported" || (l.Mode == "hybrid" && i == len(l.EndpointIDs)-1)) && e.CredentialID == "" {
			return nil, badRequest("导入的端点需要填写线路密钥")
		}
		if e.OwnerID != "" {
			if len(l.UserIDs) != 1 || l.UserIDs[0] != e.OwnerID {
				return nil, badRequest("私有落地只能用于限定了该用户的线路")
			}
		}
		if l.Mode == "hybrid" && i == len(l.EndpointIDs)-1 && e.Capabilities.Managed {
			return nil, badRequest("“平台入口＋自带落地”模式下，落地端点不能由平台管理")
		}
		hops = append(hops, c.Hop{EndpointID: e.ID, NodeID: n.ID, Kind: e.Kind, Host: e.Host, Port: e.Port, PortEnd: e.PortEnd, Cipher: e.Cipher, CredentialID: e.CredentialID, Capabilities: e.Capabilities})
	}
	for _, gameID := range l.GameIDs {
		var game Game
		if tx.First(&game, "id = ?", gameID).Error != nil {
			return nil, badRequest("限定游戏中有不存在的游戏")
		}
		if game.Draft.UDP {
			for _, h := range hops {
				if !h.Capabilities.UDP {
					return nil, badRequest("游戏 " + gameID + " 需要 UDP，但线路中有一跳不支持 UDP")
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

// egressAllowedBatch applies egressAllowed to many exits in two queries.
func egressAllowedBatch(db *gorm.DB, endpointIDs []string, userID string) (map[string]bool, error) {
	out := map[string]bool{}
	var endpoints []Endpoint
	if e := db.Where("id IN ?", endpointIDs).Find(&endpoints).Error; e != nil {
		return nil, e
	}
	var dedicated []string
	if e := db.Model(&Binding{}).Where("endpoint_id IN ? AND enabled = ? AND kind = ? AND user_id <> ?", endpointIDs, true, "dedicated", userID).Pluck("endpoint_id", &dedicated).Error; e != nil {
		return nil, e
	}
	for _, ep := range endpoints {
		out[ep.ID] = ep.Enabled && (ep.OwnerID == "" || ep.OwnerID == userID) && !slices.Contains(dedicated, ep.ID)
	}
	return out, nil
}
