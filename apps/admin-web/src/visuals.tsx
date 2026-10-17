import { useState, type ReactNode } from "react";
import { stateNames } from "./format";

/** Readable names for the region codes games and lines use. */
export const regionNames: Record<string, string> = {
  kr: "韩服",
  jp: "日服",
  tw: "台服",
  hk: "港服",
  sg: "新加坡",
  sea: "东南亚",
  na: "美服",
  us: "美服",
  eu: "欧服",
  cn: "国服",
  br: "巴西",
  latam: "拉美",
  oce: "大洋洲",
  ru: "俄服",
  global: "国际服",
};

/** Regions offered as one-click additions in the editor. */
export const commonRegions = ["kr", "jp", "tw", "hk", "sg", "sea", "na", "eu"];

export const regionLabel = (code: string) =>
  regionNames[code.toLowerCase()] || code;

// Region badge colours by position in a row's list, as in the client:
// consecutive entries are at least 100 degrees apart.
const regionHues = [205, 345, 155, 25, 270, 120];

/** A region code badge. A negative slot is a region not chosen yet: grey. */
export function RegionBadge({ code, slot }: { code: string; slot: number }) {
  return (
    <b
      className={"region-code" + (slot < 0 ? " idle" : "")}
      style={slot >= 0 ? { background: `hsl(${regionHues[slot % regionHues.length]} 45% 42%)` } : undefined}
    >
      {code.slice(0, 3).toUpperCase()}
    </b>
  );
}

/** A region as its badge and Chinese name, the same in lists and editors. */
export function RegionName({ code, slot }: { code: string; slot: number }) {
  const name = regionNames[code.toLowerCase()];
  return (
    <>
      <RegionBadge code={code} slot={slot} />
      {name || null}
    </>
  );
}

export function RegionChips({ codes }: { codes: unknown }) {
  const list = Array.isArray(codes) ? (codes as string[]) : [];
  if (!list.length) return <span className="muted">待配置</span>;
  return (
    <span className="chips">
      {list.map((c, i) => (
        <span className="chip" key={c}>
          <RegionName code={c} slot={i} />
        </span>
      ))}
    </span>
  );
}

// FNV-1a: the same id always gets the same colours.
function hash(text: string) {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

// The client's cover palette (Controls.cs): hues that stay clear at a dark
// cover's lightness, ordered so consecutive entries are far apart.
const coverHues = [222, 340, 168, 22, 252, 140, 312, 192, 6];

/**
 * A game's cover hue, computed as the client does: from the control
 * service's cover index when there is one, otherwise from the id.
 */
export function coverHue(seed: string, index?: unknown) {
  if (typeof index === "number" && index >= 0) return coverHues[index % coverHues.length];
  const h = hash(seed);
  return coverHues[h % coverHues.length] + ((h >>> 8) % 11) - 5;
}

/** Where the control service serves an uploaded cover picture. */
export function coverUrl(image: unknown) {
  if (typeof image !== "string") return undefined;
  if (image.startsWith("data:image/")) return image;
  return /^[0-9a-f]{64}$/.test(image) ? "/api/client/v1/covers/" + image : undefined;
}

/**
 * A game's cover: the picture an operator uploaded, or generated art (its
 * own hue, a soft light and streaks, the first character set faint). No
 * game artwork is bundled; a picture that fails to load falls back.
 */
export function Cover({
  seed,
  name,
  index,
  image,
  banner,
  width = 64,
  height = 36,
}: {
  seed: string;
  name: string;
  /** The control service's cover_index for the game. */
  index?: unknown;
  /** The game's cover_image, or a data: URL being previewed. */
  image?: unknown;
  /** Cropped to a wide strip, as the client's game page banner. */
  banner?: boolean;
  width?: number;
  height?: number;
}) {
  const src = coverUrl(image);
  const [failed, setFailed] = useState<string>();
  if (src && failed !== src)
    return (
      <img
        className={"cover" + (banner ? " banner" : "")}
        src={src}
        width={width}
        height={height}
        alt=""
        loading="lazy"
        decoding="async"
        onError={() => setFailed(src)}
      />
    );
  const h = hash(seed || name);
  const hue = coverHue(seed || name, index);
  const id = "cv" + h.toString(36);
  const streaks = [0, 1, 2].map((i) => {
    const r = (h >>> (i * 7)) & 127;
    return { y: 6 + ((r * 3) % 28), len: 26 + (r % 30), x: -6 + (r % 22) };
  });
  const initial = (name || "?").slice(0, 1).toUpperCase();
  return (
    <svg
      className="cover"
      width={width}
      height={height}
      viewBox="0 0 64 36"
      preserveAspectRatio="xMidYMid slice"
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={id + "g"} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor={`hsl(${hue} 52% 33%)`} />
          <stop offset="1" stopColor={`hsl(${hue + 28} 58% 11%)`} />
        </linearGradient>
        <radialGradient id={id + "r"} cx="0.82" cy="0.15" r="0.75">
          <stop offset="0" stopColor={`hsl(${hue - 12} 80% 64% / 0.59)`} />
          <stop offset="1" stopColor={`hsl(${hue - 12} 80% 64% / 0)`} />
        </radialGradient>
        <linearGradient id={id + "s"} x1="0" y1="0" x2="1" y2="0">
          <stop offset="0" stopColor="#fff" stopOpacity="0" />
          <stop offset="0.72" stopColor={`hsl(${hue - 24} 95% 84%)`} stopOpacity="0.7" />
          <stop offset="1" stopColor="#fff" stopOpacity="0" />
        </linearGradient>
      </defs>
      <rect width="64" height="36" fill={`url(#${id}g)`} />
      <rect width="64" height="36" fill={`url(#${id}r)`} />
      <g transform="rotate(-22 32 18)">
        {streaks.map((s, i) => (
          <rect
            key={i}
            x={s.x}
            y={s.y}
            width={s.len}
            height={i === 1 ? 1.6 : 1}
            rx="0.8"
            fill={`url(#${id}s)`}
          />
        ))}
      </g>
      <text
        x="61"
        y="33"
        textAnchor="end"
        fontSize="26"
        fontWeight="700"
        fill="#fff"
        fillOpacity="0.15"
        fontFamily="Bahnschrift, Microsoft YaHei UI, sans-serif"
      >
        {initial}
      </text>
    </svg>
  );
}

/**
 * A person's initial on a soft colour of their own, taken from the cover
 * palette: identity, like a game's cover, never one of the state tones.
 */
export function Avatar({ name, small }: { name: string; small?: boolean }) {
  const hue = coverHues[hash(name) % coverHues.length];
  return (
    <span
      className={"avatar" + (small ? " xs" : "")}
      style={{ background: `hsl(${hue} 42% 92%)`, color: `hsl(${hue} 48% 32%)` }}
      aria-hidden="true"
    >
      {(name || "?").slice(0, 1).toUpperCase()}
    </span>
  );
}

/** Rows and their name cell: a picture or icon, a name and a short line. */
export function Entity({
  lead,
  title,
  sub,
}: {
  lead: ReactNode;
  title: ReactNode;
  sub?: ReactNode;
}) {
  return (
    <span className="entity">
      {lead}
      <span>
        <b>{title}</b>
        {sub ? <small>{sub}</small> : null}
      </span>
    </span>
  );
}

/** Which tone a stored state value is shown in. */
export function toneOf(value: unknown): string {
  switch (String(value)) {
    case "ACTIVE":
    case "succeeded":
    case "ready":
    case "true":
      return "tone-good";
    case "PREPARING":
    case "CONNECTING":
    case "RECONNECTING":
    case "STOPPING":
    case "validating":
    case "pending":
    case "running":
      return "tone-warn";
    case "DEGRADED":
    case "failed":
      return "tone-bad";
    default:
      return "tone-neutral";
  }
}

export function Pill({
  value,
  label,
  tone,
  plain,
}: {
  value?: unknown;
  label?: ReactNode;
  tone?: string;
  plain?: boolean;
}) {
  const shown = label ?? stateNames[String(value)] ?? String(value);
  return (
    <span className={"pill " + (tone || toneOf(value)) + (plain ? " plain" : "")}>
      {shown}
    </span>
  );
}

/** "3 分钟前"; the exact time is in the title. */
export function Ago({ at }: { at: unknown }) {
  if (!at || typeof at !== "string" || at.startsWith("0001-"))
    return <span className="muted">—</span>;
  const t = new Date(at);
  const seconds = (Date.now() - t.getTime()) / 1000;
  const exact = t.toLocaleString("zh-CN", { hour12: false });
  let text: string;
  if (seconds < 0) {
    const ahead = -seconds;
    text =
      ahead < 60
        ? "1 分钟内"
        : ahead < 3600
          ? Math.round(ahead / 60) + " 分钟后"
          : ahead < 86400
            ? Math.round(ahead / 3600) + " 小时后"
            : exact;
  } else
    text =
      seconds < 45
        ? "刚刚"
        : seconds < 3600
          ? Math.max(1, Math.round(seconds / 60)) + " 分钟前"
          : seconds < 86400
            ? Math.round(seconds / 3600) + " 小时前"
            : seconds < 86400 * 30
              ? Math.round(seconds / 86400) + " 天前"
              : exact;
  return <span title={exact}>{text}</span>;
}

/**
 * How long the control service waits for a heartbeat before it treats the
 * node as offline (NodeTimeout there) and stops placing sessions on it.
 */
const NODE_TIMEOUT_S = 45;

/** Heartbeat age as a coloured dot plus the age. */
export function Heartbeat({ at }: { at: unknown }) {
  if (!at || typeof at !== "string" || at.startsWith("0001-"))
    return <Pill tone="tone-neutral" label="从未上线" />;
  const age = (Date.now() - new Date(at).getTime()) / 1000;
  const online = age <= NODE_TIMEOUT_S;
  return (
    <span
      className={"pill " + (online ? "tone-good" : age <= 300 ? "tone-warn" : "tone-neutral")}
      title={online ? "在线" : "超过 " + NODE_TIMEOUT_S + " 秒没有心跳，已视为离线，不会分配新会话"}
    >
      <Ago at={at} />
    </span>
  );
}
