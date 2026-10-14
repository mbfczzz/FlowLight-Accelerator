import { ChevronRight, Network } from "lucide-react";
import type { Row } from "../api";

const metrics = [
  ["games", "游戏规则"],
  ["active_sessions", "当前会话"],
  ["online_nodes", "在线节点"],
  ["verified_versions", "已验证线路版本"],
] as const;

function metricNote(key: string) {
  if (key === "online_nodes") return "依据最近心跳";
  if (key === "verified_versions") return "依据实际 TCP / UDP 探测";
  return "控制服务记录";
}

export function Overview({
  overview,
  loading,
  onOpenNodes,
}: {
  overview: Row;
  loading: boolean;
  onOpenNodes: () => void;
}) {
  return (
    <>
      <div className="metrics">
        {metrics.map(([k, n]) => (
          <div key={k}>
            <span>{n}</span>
            <strong>{loading ? "—" : (overview[k] ?? "—")}</strong>
            <small>{metricNote(k)}</small>
          </div>
        ))}
      </div>
      <div className="onboarding">
        <div>
          <Network size={28} />
          <h2>从第一条可用线路开始</h2>
          <p>
            登记节点与服务端点，为游戏配置区服，组合线路并发布。节点完成执行与探测后，客户端才能选择该线路。
          </p>
          <button className="primary" onClick={onOpenNodes}>
            管理转发节点
            <ChevronRight size={16} />
          </button>
        </div>
        <ol>
          <li>
            <b>01</b>
            <span>
              配置游戏规则
              <small>VALORANT 与英雄联盟已提供初始草稿</small>
            </span>
          </li>
          <li>
            <b>02</b>
            <span>
              接入节点与端点
              <small>按节点配置独立身份、协议和端口池</small>
            </span>
          </li>
          <li>
            <b>03</b>
            <span>
              发布并验证线路<small>实际探测通过后开放新会话</small>
            </span>
          </li>
        </ol>
      </div>
    </>
  );
}
