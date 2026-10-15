import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { ChartColumn, Gamepad2, HardDrive, Route, Server, TriangleAlert, Users, Zap } from "lucide-react";
import { api, isSessionExpired, listPage, MAX_PAGE_SIZE, type Row } from "../api";
import { formatBytes } from "../format";
import { messageOf } from "../useSectionData";
import { Avatar, Cover } from "../visuals";
import { Segmented } from "./controls";

/**
 * Traffic as the exit servers measured it (upload and download together),
 * by local day, or by hour for today, with the heaviest players, lines,
 * games and servers. One series, so one hue: a deeper amber than the
 * brand's (#e6ae45 is 2:1 on white), validated at 3:1 for marks.
 */
const MARK = "#c4881c";
const MARK_HOVER = "#d39a2c";

const ranges: [string, string][] = [
  ["1", "今天"],
  ["7", "近 7 天"],
  ["30", "近 30 天"],
  ["90", "近 90 天"],
];

type Point = { start: string; bytes: number };
type Share = { id: string; bytes: number; sessions: number };
type Report = {
  days: number;
  /** The zone the buckets were cut in, minutes east of UTC. */
  tz: number;
  series: Point[];
  totals: { bytes: number; sessions: number; users: number };
  users: Share[];
  lines: Share[];
  games: Share[];
  nodes: Share[];
};

/** Counting needs node agent 0.2.0 or later. */
const COUNTING_AGENT = "0.2.0";
function older(a: string, b: string) {
  const x = a.split(/[.+-]/).map((n) => parseInt(n, 10) || 0);
  const y = b.split(/[.+-]/).map((n) => parseInt(n, 10) || 0);
  for (let i = 0; i < 3; i++) if ((x[i] || 0) !== (y[i] || 0)) return (x[i] || 0) < (y[i] || 0);
  return false;
}

/** A clean axis in one binary unit: 0 / 250 MB / 500 MB / 750 MB. */
function axis(max: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let unit = 0;
  while (unit < units.length - 1 && max >= 1024 ** (unit + 1)) unit++;
  const scale = 1024 ** unit;
  const top = Math.max(max / scale, 1);
  const raw = top / 4;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * pow).find((s) => s >= raw) || 10 * pow;
  const ticks: number[] = [];
  for (let v = 0; v <= top + step * 0.001; v += step) ticks.push(v);
  if (ticks[ticks.length - 1] < top) ticks.push(ticks[ticks.length - 1] + step);
  const label = (v: number) => (v === 0 ? "0" : (Number.isInteger(v) ? v : v.toFixed(1)) + " " + units[unit]);
  return { max: ticks[ticks.length - 1] * scale, ticks: ticks.map((v) => ({ value: v * scale, label: label(v) })) };
}

/** The bucket's wall-clock time in the report's zone, read with UTC getters. */
function local(p: Point, tz: number) {
  return new Date(Date.parse(p.start) + tz * 60000);
}

function pointLabel(p: Point, hourly: boolean, tz: number) {
  const d = local(p, tz);
  return hourly
    ? d.getUTCHours() + ":00–" + (d.getUTCHours() + 1) + ":00"
    : d.getUTCMonth() + 1 + "月" + d.getUTCDate() + "日";
}

function tickLabel(p: Point, hourly: boolean, tz: number) {
  const d = local(p, tz);
  return hourly ? d.getUTCHours() + "时" : d.getUTCMonth() + 1 + "/" + d.getUTCDate();
}

/** A rounded-top column standing on the baseline. */
function column(x: number, w: number, top: number, base: number) {
  const r = Math.min(4, w / 2, base - top);
  return `M${x},${base}V${top + r}Q${x},${top} ${x + r},${top}H${x + w - r}Q${x + w},${top} ${x + w},${top + r}V${base}Z`;
}

function ColumnChart({ points, hourly, tz }: { points: Point[]; hourly: boolean; tz: number }) {
  const box = useRef<HTMLDivElement>(null);
  // 0 until measured: a guessed width would widen the page on phones.
  const [width, setWidth] = useState(0);
  const [active, setActive] = useState<number | null>(null);
  // One tab stop for the chart; the arrow keys move between columns.
  const [focusIndex, setFocusIndex] = useState(0);
  const svg = useRef<SVGSVGElement>(null);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const measure = () => setWidth(Math.max(240, el.clientWidth));
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const height = 248;
  const left = 64;
  const right = 8;
  const top = 22;
  const bottom = 28;
  const plotW = width - left - right;
  const base = height - bottom;
  const peak = points.reduce((m, p) => Math.max(m, p.bytes), 0);
  const { max, ticks } = axis(peak);
  const slot = plotW / Math.max(points.length, 1);
  // At most 24px wide, with a 2px gap between neighbours.
  const barW = Math.max(1, Math.min(24, slot - 2));
  const y = (v: number) => base - (v / max) * (base - top);
  const every = Math.max(1, Math.ceil(46 / slot));
  const peakIndex = points.findIndex((p) => p.bytes === peak && peak > 0);
  const shown = active !== null ? points[active] : null;
  const move = (to: number) => {
    const next = Math.max(0, Math.min(points.length - 1, to));
    setFocusIndex(next);
    svg.current?.querySelectorAll<SVGRectElement>("rect[data-slot]")[next]?.focus();
  };

  if (!width) return <div className="chart" ref={box} style={{ height }} />;
  return (
    <div className="chart" ref={box}>
      <svg ref={svg} width={width} height={height} role="group" aria-label={"流量" + (hourly ? "（按小时）" : "（按天）") + "，用左右方向键逐个查看"}>
        {ticks.map((t) => (
          <g key={t.value}>
            <line x1={left} x2={width - right} y1={y(t.value)} y2={y(t.value)} className={t.value === 0 ? "chart-base" : "chart-grid"} />
            <text x={left - 10} y={y(t.value)} className="chart-axis" textAnchor="end" dominantBaseline="middle">
              {t.label}
            </text>
          </g>
        ))}
        {points.map((p, i) => {
          const x = left + i * slot + (slot - barW) / 2;
          return (
            <g key={p.start}>
              {p.bytes > 0 && (
                <path d={column(x, barW, y(p.bytes), base)} fill={active === i ? MARK_HOVER : MARK} />
              )}
              {i % every === 0 && (
                <text x={left + i * slot + slot / 2} y={height - 8} className="chart-axis" textAnchor="middle">
                  {tickLabel(p, hourly, tz)}
                </text>
              )}
              {i === peakIndex && (
                <text x={left + i * slot + slot / 2} y={y(p.bytes) - 7} className="chart-peak" textAnchor="middle">
                  {formatBytes(p.bytes)}
                </text>
              )}
              {/* The whole column slot answers hover and focus, not only the bar. */}
              <rect
                x={left + i * slot}
                y={top}
                width={slot}
                height={base - top}
                fill="transparent"
                data-slot=""
                role="img"
                tabIndex={i === Math.min(focusIndex, points.length - 1) ? 0 : -1}
                aria-label={pointLabel(p, hourly, tz) + " " + formatBytes(p.bytes)}
                onPointerEnter={() => setActive(i)}
                onPointerLeave={() => setActive((a) => (a === i ? null : a))}
                onFocus={() => {
                  setFocusIndex(i);
                  setActive(i);
                }}
                onBlur={() => setActive((a) => (a === i ? null : a))}
                onKeyDown={(e) => {
                  const to =
                    e.key === "ArrowRight" ? i + 1 : e.key === "ArrowLeft" ? i - 1 : e.key === "Home" ? 0 : e.key === "End" ? points.length - 1 : null;
                  if (to === null) return;
                  e.preventDefault();
                  move(to);
                }}
              />
            </g>
          );
        })}
      </svg>
      {shown && active !== null && (
        <div
          className={"chart-tip" + (y(shown.bytes) < 64 ? " below" : "")}
          style={{
            left: Math.min(Math.max(left + active * slot + slot / 2, 70), width - 70),
            top: y(shown.bytes) < 64 ? y(shown.bytes) + 10 : y(shown.bytes) - 12,
          }}
        >
          <b>{formatBytes(shown.bytes)}</b>
          <span>{pointLabel(shown, hourly, tz)}</span>
        </div>
      )}
    </div>
  );
}

function Ranking({
  title,
  icon,
  rows,
  lead,
  name,
}: {
  title: string;
  icon: ReactNode;
  rows: Share[];
  lead: (r: Share) => ReactNode;
  name: (r: Share) => string;
}) {
  const top = rows[0]?.bytes || 1;
  return (
    <section className="panel ranking">
      <h3>
        {icon}
        {title}
      </h3>
      {rows.length === 0 ? (
        <p className="muted">这段时间没有记录</p>
      ) : (
        <ol>
          {rows.map((r) => (
            <li key={r.id || "none"}>
              <div className="ranking-row">
                {lead(r)}
                <span className="ranking-name" title={name(r)}>
                  {name(r)}
                </span>
                <span className="ranking-meta">{r.sessions} 次</span>
                <b className="figure">{formatBytes(r.bytes)}</b>
              </div>
              <span className="ranking-bar" aria-hidden="true">
                <i style={{ width: Math.max(2, (r.bytes / top) * 100) + "%", background: MARK }} />
              </span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

export function Traffic({
  names,
  covers,
  tick,
}: {
  names: Record<string, string>;
  covers: Record<string, { index?: number; image?: string }>;
  /** Changes when the page head's 刷新 is pressed. */
  tick: number;
}) {
  const [days, setDays] = useState("7");
  const [report, setReport] = useState<Report | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [outdated, setOutdated] = useState<Row[]>([]);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError("");
    api("/traffic?days=" + days + "&tz=" + -new Date().getTimezoneOffset())
      .then((r) => alive && setReport(r))
      .catch((e) => alive && !isSessionExpired(e) && setError(messageOf(e)))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [days, tick]);

  useEffect(() => {
    let alive = true;
    listPage("nodes", MAX_PAGE_SIZE, 0)
      .then((rows) => alive && setOutdated(rows.filter((n) => n.agent_version && older(String(n.agent_version), COUNTING_AGENT))))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [tick]);

  const nameOf = (id: string) => names[id] || (id ? id.slice(0, 8) : "—");
  const hourly = report ? report.days === 1 : days === "1";
  const tz = report?.tz ?? -new Date().getTimezoneOffset();
  const totals = report?.totals;
  const average = totals && totals.sessions ? totals.bytes / totals.sessions : 0;
  const empty = !!report && !totals?.bytes;
  const tiles: [string, string, ReactNode, string][] = [
    ["总流量", totals ? formatBytes(totals.bytes) : "—", <HardDrive size={17} />, "上传与下载合计"],
    ["加速会话", totals ? String(totals.sessions) : "—", <Zap size={17} />, "产生了流量的会话"],
    ["使用的玩家", totals ? String(totals.users) : "—", <Users size={17} />, "这段时间加速过的账号"],
    ["平均每次会话", totals ? formatBytes(average) : "—", <ChartColumn size={17} />, "总流量 ÷ 会话数"],
  ];

  return (
    <div className={"traffic" + (loading && report ? " is-loading" : "")}>
      <div className="traffic-filters">
        <Segmented label="时间范围" value={days} onChange={setDays} options={ranges} />
        <span className="muted">平台托管的落地服务器每 10 秒上报一次，按你电脑的时区汇总；自带落地的线路没有流量数据</span>
      </div>
      {error && (
        <div className="alert" role="alert">
          <TriangleAlert size={16} />
          <span>{error}</span>
        </div>
      )}
      {outdated.length > 0 && (
        <div className="callout warn">
          <TriangleAlert size={16} />
          <span>
            这些节点服务器的节点程序低于 {COUNTING_AGENT}，不会上报流量：
            {outdated.map((n) => n.name + "（" + n.agent_version + "）").join("、")}。在服务器上用不带凭据的“接入命令”再运行一次即可更新。
          </span>
        </div>
      )}
      <div className="tiles">
        {tiles.map(([label, value, icon, note]) => (
          <div className="tile" key={label}>
            <div className="tile-head">
              <span className="tile-icon tone-neutral">{icon}</span>
              {label}
            </div>
            <strong>{loading && !report ? "—" : value}</strong>
            <small>{note}</small>
          </div>
        ))}
      </div>
      <section className="panel chart-panel">
        <div className="chart-head">
          <h2>{hourly ? "今天每小时的流量" : "每天的流量"}</h2>
          {report && !empty && (
            <span className="muted">
              {ranges.find(([v]) => v === String(report.days))?.[1]}共 {formatBytes(totals?.bytes)}
            </span>
          )}
        </div>
        {!report ? (
          <div className="empty">
            <h3>{loading ? "正在读取…" : "未能读取流量"}</h3>
          </div>
        ) : empty ? (
          <div className="empty">
            <span className="empty-icon">
              <ChartColumn size={26} />
            </span>
            <h3>这段时间还没有流量记录</h3>
            <p>流量由落地服务器上的节点程序上报（需要 {COUNTING_AGENT} 或更高版本），玩家开始加速后几十秒内就会出现在这里。</p>
          </div>
        ) : (
          <>
            <ColumnChart points={report.series} hourly={hourly} tz={tz} />
            <details className="chart-table">
              <summary>查看明细</summary>
              <table>
                <thead>
                  <tr>
                    <th>{hourly ? "时段" : "日期"}</th>
                    <th>流量</th>
                  </tr>
                </thead>
                <tbody>
                  {report.series.map((p) => (
                    <tr key={p.start}>
                      <td>{pointLabel(p, hourly, tz)}</td>
                      <td className="figure">{formatBytes(p.bytes)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </details>
          </>
        )}
      </section>
      {report && !empty && (
        <div className="rankings">
          <Ranking title="玩家" icon={<Users size={16} />} rows={report.users} name={(r) => nameOf(r.id)} lead={(r) => <Avatar name={nameOf(r.id)} small />} />
          <Ranking title="线路" icon={<Route size={16} />} rows={report.lines} name={(r) => nameOf(r.id)} lead={() => <span className="ranking-icon"><Route size={13} /></span>} />
          <Ranking
            title="游戏"
            icon={<Gamepad2 size={16} />}
            rows={report.games}
            name={(r) => nameOf(r.id)}
            lead={(r) => <Cover seed={r.id} name={nameOf(r.id)} index={covers[r.id]?.index} image={covers[r.id]?.image} width={30} height={17} />}
          />
          <Ranking title="落地服务器" icon={<Server size={16} />} rows={report.nodes} name={(r) => nameOf(r.id)} lead={() => <span className="ranking-icon"><Server size={13} /></span>} />
        </div>
      )}
    </div>
  );
}
