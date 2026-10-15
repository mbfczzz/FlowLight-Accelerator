---
name: FlowLight Accelerator
description: Light operational administration and a dark graphite Windows game client lit by one amber accent.
colors:
  admin-accent: "#087e93"
  admin-background: "#f6f8fa"
  admin-surface: "#ffffff"
  admin-text: "#182433"
  admin-muted: "#677487"
  admin-line: "#e2e7ec"
  success-background: "#e5f4ed"
  success-text: "#216944"
  failure-background: "#fff0ed"
  failure-text: "#ac4934"
  progress-background: "#e8f2fc"
  progress-text: "#34638d"
  client-background: "#131217"
  client-sidebar: "#0D0C10"
  client-surface: "#1B1A21"
  client-elevated: "#23212A"
  client-hover: "#2B2833"
  client-border: "#2F2C38"
  client-border-strong: "#3D3948"
  client-text: "#F3F0F6"
  client-text-secondary: "#B3ADBE"
  client-text-tertiary: "#948E9E"
  client-accent: "#E6AE45"
  client-accent-hover: "#F0BD5C"
  client-accent-pressed: "#D39D36"
  client-accent-soft: "#2C2518"
  client-on-accent: "#1C1300"
  client-good: "#5CC98B"
  client-warn: "#EE9A3C"
  client-bad: "#E4606A"
  client-info: "#7AA6E8"
typography:
  admin-body:
    fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif'
    fontSize: "13px"
    lineHeight: 1.8
  admin-label:
    fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif'
    fontSize: "12px"
  client-headline:
    fontFamily: "Microsoft YaHei UI, Segoe UI, Microsoft YaHei"
    fontSize: "22px"
    fontWeight: 600
  client-title:
    fontFamily: "Microsoft YaHei UI, Segoe UI, Microsoft YaHei"
    fontSize: "15px"
    fontWeight: 600
  client-body:
    fontFamily: "Microsoft YaHei UI, Segoe UI, Microsoft YaHei"
    fontSize: "13px"
  client-meta:
    fontFamily: "Microsoft YaHei UI, Segoe UI, Microsoft YaHei"
    fontSize: "12.5px"
  client-label:
    fontFamily: "Microsoft YaHei UI, Segoe UI, Microsoft YaHei"
    fontSize: "11.5px"
  client-number-display:
    fontFamily: "Bahnschrift, Segoe UI Semibold, Microsoft YaHei UI"
    fontSize: "48px"
  client-number:
    fontFamily: "Bahnschrift, Segoe UI Semibold, Microsoft YaHei UI"
    fontSize: "26px"
  client-number-row:
    fontFamily: "Bahnschrift, Segoe UI Semibold, Microsoft YaHei UI"
    fontSize: "19px"
rounded:
  chip: "4px"
  control: "7px"
  container: "9px"
  dialog: "12px"
  client-control: "8px"
  client-choice: "10px"
  client-panel: "12px"
  client-hero: "14px"
spacing:
  compact: "8px"
  control-gap: "10px"
  label-gap: "12px"
  section-gap: "20px"
  section: "24px"
  desktop-content: "36px"
  client-row-gap: "8px"
  client-tile-gap: "10px"
  client-card-gap: "16px"
  client-gutter: "20px"
  client-page-game: "24px"
  client-page: "32px"
components:
  button-primary:
    backgroundColor: "{colors.admin-accent}"
    textColor: "{colors.admin-surface}"
    rounded: "{rounded.control}"
    padding: "10px 15px"
  button-secondary:
    backgroundColor: "{colors.admin-surface}"
    textColor: "{colors.admin-text}"
    rounded: "{rounded.control}"
    padding: "10px 15px"
  input:
    backgroundColor: "{colors.admin-surface}"
    textColor: "{colors.admin-text}"
    rounded: "{rounded.control}"
    padding: "10px 12px"
  navigation-item:
    padding: "11px 12px"
    rounded: "{rounded.control}"
  badge-ready:
    backgroundColor: "{colors.success-background}"
    textColor: "{colors.success-text}"
    rounded: "{rounded.chip}"
    padding: "4px 7px"
  resource-table:
    backgroundColor: "{colors.admin-surface}"
    rounded: "{rounded.container}"
  button-client:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-text}"
    rounded: "{rounded.client-control}"
    padding: "8px 14px"
    height: "36px"
  button-client-hover:
    backgroundColor: "{colors.client-hover}"
  button-client-primary:
    backgroundColor: "{colors.client-accent}"
    textColor: "{colors.client-on-accent}"
    rounded: "{rounded.client-control}"
    padding: "8px 14px"
    height: "42px"
  button-client-primary-hover:
    backgroundColor: "{colors.client-accent-hover}"
  button-client-primary-pressed:
    backgroundColor: "{colors.client-accent-pressed}"
  button-client-primary-disabled:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-text-tertiary}"
  input-client:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-text}"
    rounded: "{rounded.client-control}"
    padding: "0 10px"
    height: "40px"
  nav-rail-item:
    textColor: "{colors.client-text-tertiary}"
    rounded: "{rounded.client-panel}"
    size: "60px"
  nav-rail-item-selected:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-accent}"
  game-card:
    backgroundColor: "{colors.client-surface}"
    rounded: "{rounded.client-panel}"
    padding: "8px"
  game-card-hover:
    backgroundColor: "{colors.client-elevated}"
  region-tile:
    backgroundColor: "{colors.client-surface}"
    rounded: "{rounded.client-choice}"
    padding: "0 12px"
    width: "128px"
    height: "62px"
  region-tile-selected:
    backgroundColor: "{colors.client-accent-soft}"
  line-row:
    backgroundColor: "{colors.client-surface}"
    rounded: "{rounded.client-choice}"
    padding: "8px 16px"
    height: "60px"
  line-row-selected:
    backgroundColor: "{colors.client-accent-soft}"
  chip-recommended:
    backgroundColor: "{colors.client-accent-soft}"
    textColor: "{colors.client-accent}"
    rounded: "{rounded.chip}"
    padding: "1px 6px"
  stat-tile:
    backgroundColor: "{colors.client-surface}"
    rounded: "{rounded.client-panel}"
    padding: "14px 16px"
  hero-banner:
    rounded: "{rounded.client-hero}"
    height: "184px"
  dial-ready:
    backgroundColor: "{colors.client-accent}"
    textColor: "{colors.client-on-accent}"
    size: "168px"
  dial-idle:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-text-tertiary}"
    size: "168px"
  toast-client:
    backgroundColor: "{colors.client-elevated}"
    textColor: "{colors.client-text}"
    rounded: "{rounded.client-choice}"
    padding: "10px 18px 10px 12px"
---

# Design System: FlowLight Accelerator

## Overview

**Creative North Star: "Operate"**

FlowLight has two surfaces with one job each. Operators work in a light administration of compact resource tables, persistent navigation and modal editors, where deep cyan marks actions and selection. Players use a dark native Windows client that sits beside a running game, usually in the evening: a game library rather than a settings form. Generated cover art opens each game, region and line choices are tiles and rows that carry their own evidence, and one round dial starts acceleration and then becomes the live latency readout.

The client's material is warm graphite with a faint violet cast, separated by tone rather than outlines and lit by a single amber, the "flowing light" of the product's name. Amber means brand, action, selection and keyboard focus; latency quality and session state speak in a separate green, orange, red and blue set. Pictures carry recognition: cover art drawn per game from a hue and a few light streaks, outline icons on one 24 px grid, four signal bars, a latency chart. No game artwork is bundled, and no figure appears without saying what it measures.

**Key Characteristics:**
- Light administration with deep cyan actions; dark graphite client with one amber accent.
- Client: 80 DIP icon rail, a library of 16:9 cover cards, a game page with a hero banner, region tiles, line rows and the dial.
- Every client figure is a probe with a visible provenance label; missing evidence is an em dash.
- Unknown, draft, unavailable, refused, historical and current states remain explicit.

## Colors

The palette splits by surface: administration is cool white with a deep cyan; the client is a five-step graphite ladder with one warm amber and four semantic state hues.

### Primary

- **Operator Cyan** (admin-accent): primary buttons, inline actions and navigation cues in the administration. It never appears in the client.
- **Flowing-Light Amber** (client-accent): the client's only accent. It fills the brand mark, the ready dial (as a gradient from light gold #F7CB72 through the accent to burnt amber #CC8C2B), and primary buttons; it marks the selected rail item (icon, label and a 3 DIP edge bar), the selected region tile and line row (1.5 DIP edge plus tick or radio dot), section-title icons, region codes on the hero, the 推荐 chip, the avatar, the text caret and every keyboard focus ring. Hover lifts it to a lighter gold (client-accent-hover); press deepens it (client-accent-pressed). Text on amber is near-black brown (client-on-accent), never white.
- **Amber Wash** (client-accent-soft): a solid dark amber-brown that fills selected tiles and rows, the avatar disc and the 推荐 chip. The ready dial's track and the avatar ring use the accent at 35% alpha.

### Secondary

Client state colors. Each also has a 14% alpha tint used only as the round badge behind an icon (dashboard tiles, toasts).

- **Signal Green** (client-good): latency of 60 ms or less, load under 50%, an active connection, the 加速中 badge on a cover, the rail dot and taskbar overlay while a session runs.
- **Ember Orange** (client-warn): latency of 61 to 120 ms, load of 50 to 80%, preparing, connecting, reconnecting, degraded and stopping; the packet-loss tile icon.
- **Brake Red** (client-bad): latency above 120 ms, load of 80% or more, refused lines, failed or blocked sessions, inline and sign-in errors.
- **Probe Blue** (client-info): the jitter tile and informational toasts.

### Neutral

Admin background surrounds white surfaces; dark text, muted supporting text and fine dividers establish hierarchy. Success, failure and progress color pairs are state indicators in administration badges; labels carry the state alongside color.

The client's neutrals form a tone ladder, darkest to lightest: **Rail Black** (client-sidebar) for the icon rail, **Night Graphite** (client-background) for the window, **Graphite Surface** (client-surface) for cards, panels, tiles and rows, **Raised Graphite** (client-elevated) for hover, secondary buttons, fields, the idle dial, toasts and the account menu, and **Lift** (client-hover) for hover on raised items. **Seam** (client-border) draws dividers, the idle dial track and chart gridlines; **Strong Seam** (client-border-strong) draws unselected radio rings, field hover edges, unlit signal bars and scrollbar thumbs. **Moon Text** (client-text) is primary ink; **Dusk Text** (client-text-secondary) carries meta lines and labels; **Ash Text** (client-text-tertiary) carries captions, units, unknown states and disabled ink.

Scrims are the rail tone at partial alpha: 55% under the back pill on art, 70% under the hero status pill, 88% behind the sign-in dialog; cover badges sit on black at 60%.

### Named Rules

**The One Amber Rule.** Amber marks the brand, the action, the selection and keyboard focus, and nothing else. It never reports quality: latency, load and connection state speak only in green, orange, red and blue.

**The Evidence Rule.** A color, bar count or figure must describe reported state: an enabled resource, a published profile, a verified path, a line's historical probe and a live tunnel probe are separate facts. A line without a measurement shows no lit bars, a tertiary em dash and no unit; a line with unknown capacity says 负载未知 in tertiary rather than guessing a load color.

## Typography

**Admin Font:** Segoe UI (with Microsoft YaHei, sans-serif)
**Client Interface Font:** Microsoft YaHei UI (with Segoe UI, Microsoft YaHei)
**Client Number Font:** Bahnschrift (with Segoe UI Semibold, Microsoft YaHei UI)

**Character:** All faces are local Windows faces; no font files are shipped or requested. In the client, YaHei UI keeps Chinese and Latin labels even at small sizes, and Bahnschrift's narrow DIN-like figures give every measurement an instrument read and keep columns of milliseconds compact.

Web tables and supporting labels use compact type (11–13px); standard prose and actions use body-sized type (13px), with prose line height (1.8). Page headings are (27px), reducing to (23px) on narrow screens; section headings are (20px). Overview numerals use tabular figures (34px), reducing to (28px) on mobile.

### Hierarchy

Client sizes are DIPs.

- **Dial figure** (Bahnschrift, 48): the live latency inside the dial.
- **Stat figure** (Bahnschrift, 26): dashboard latency, jitter, loss and elapsed time; the library banner's latency.
- **Row figure** (Bahnschrift, 19): a line's historical latency, colored by quality.
- **Headline** (SemiBold, 22; 24 in the sign-in form): page titles, 游戏库 and 登录.
- **Title** (SemiBold, 15): 选择区服, 选择线路, 延迟走势, each led by a 17 DIP amber outline icon; also card and banner names.
- **Body** (Regular, 13): the window default, buttons and summary values; primary buttons and fields step up to 14.
- **Meta** (Regular, 12.5, Dusk Text): supporting lines, stat labels, empty-state explanations.
- **Label** (Regular, 11.5, Ash Text): provenance captions, units, load text and region lists on cards. Rail labels and cover badges are 11.

Larger moments reuse the interface face at weight: the hero game title (Bold, 30, white on art), the sign-in brand name (Bold, 24), empty-state titles (SemiBold, 19; 17 for an empty search) and dial words (SemiBold, 18).

### Named Rules

**The Instrument Figures Rule.** Every measured or coded value (milliseconds, percent, elapsed time, exit IP, region codes, the avatar initial, the version string) is set in Bahnschrift, and words never are. The unit follows the figure at label size in Ash Text, bottom-aligned, and disappears with the figure when there is no measurement.

## Layout

Administration uses a fixed-width sidebar (232px), a flexible content column, header (70px), and content padding (36px), capped at (1720px). At a viewport width of (1000px) or less, the sidebar becomes (190px), content padding becomes (26px), and the four overview metrics become two columns.

At (680px) or less, navigation moves above content into a horizontally scrollable row, page actions stack below the title, content padding becomes (24px 16px), and editor fields become one column. Tables keep their readable minimum width (640px) inside a horizontally scrollable container. Login switches from two columns to the form alone. Dialogs fit the viewport with (16px) side clearance and a maximum height of (90vh).

The client window opens at (1120 × 750 DIPs) with a minimum of (900 × 650 DIPs). An icon rail (80 DIPs) on Rail Black holds the brand mark at top, two navigation items (60 × 60 DIPs; 加速 is enabled only once a game is chosen) and the avatar (40 DIPs) at the bottom. Content to its right is capped at (1320 DIPs) and centered, so a maximized window grows margins rather than stretching rows.

The library has page margins of (32 DIPs). Its header row carries the page title, a search field (260 DIPs) and a refresh icon button. Cards flow in as many columns of about (236 DIPs) as fit, with (16 DIP) gaps; each cover is 16:9 of the card's inner width. While a session runs, a banner panel sits above the grid with a small cover (112 × 63 DIPs), a green dot, 正在加速 and the game, region · line, the live latency and a 查看 button.

The game page has margins of (24 DIPs). A hero banner (184 DIPs high) spans the width and stays fixed; everything below scrolls vertically. Below it sits a flexible choices column, a (20 DIP) gutter and a fixed dial panel (316 DIPs) aligned to the top. The choices column stacks wrapping region tiles ((10 DIP) gaps) above line rows ((8 DIP) gaps). While a session runs or stops, the choices column is replaced by the dashboard: four stat tiles (two columns when the column is narrower than 540 DIPs), the latency chart panel (chart 170 DIPs high) and a wrapping facts strip. At the minimum window size the dial panel stays beside the choices and the page scrolls; it is a native minimum-size layout, not a mobile breakpoint.

Overlays anchor to the window: the toast at bottom right (28 and 24 DIPs in, at most 460 DIPs wide), the account menu rising from beside the avatar (248 DIPs wide), and the sign-in dialog centered (740 × 450 DIPs, a 310 DIP art panel beside the form).

## Elevation & Depth

The administration uses surface contrast and thin borders; no custom drop-shadow vocabulary is defined. The browser dialog uses a dim backdrop (`rgb(16 32 46 / 35%)`). Interaction feedback is immediate color, outline, border or opacity change; web reduced-motion preferences disable transitions.

The client is tonal. The five-step ladder from Rail Black to Lift separates rail, page, surfaces and interactive lift, and resting surfaces carry no outline. One shadow exists, reserved for layers that float above the page, and the dial adds light instead of shadow.

### Shadow Vocabulary

- **Float** (WPF `DropShadowEffect` BlurRadius 28, ShadowDepth 8, Direction 270, Opacity 0.5, black; about `0 8px 28px rgb(0 0 0 / 50%)`): the toast, the account menu, the sign-in dialog, and the amber bolt disc that appears on a cover under hover.
- **Dial glow** (an amber ellipse behind the dial under a 44 DIP blur): 32% opacity when ready; in the state color at 22% while connected; off otherwise.

### Motion

The dial ring carries a comet: an 80° arc fading from transparent to bright, rotating continuously. Its speed and color encode state: one turn per 6 s in light gold when ready, 1.1 s in amber while connecting (1.1 s in Ash Text while stopping), 3.2 s in green (orange when degraded) while connected; no comet when the dial cannot start. When Windows client-area animation is off, the arc holds still. All other feedback is an immediate tone change with no transition.

### Named Rules

**The Tone-Not-Line Rule.** Surfaces separate by tone, not outline. An edge appears only to say something: an amber 1.5 DIP edge for selection, a field's hover or focus edge, a 12% white hairline around cover art.

## Shapes

Administration controls use modest corners, resource containers slightly broader corners, and dialogs the broadest corners; compact state badges use the smallest (chip, control, container, dialog tokens). Table rows are divided by fine horizontal rules, and simple SVG icons communicate navigation, search, arrows and actions.

The client uses a soft four-step corner ladder: controls (8 DIPs) for buttons, fields, the cover inside a card, region code badges and focus rings; choices and transient layers (10 DIPs) for region tiles, line rows, the toast and the account menu; panels and cards (12 DIPs) for library cards, the dial panel, stat tiles, the chart panel and the rail item highlight; the hero (14 DIPs) for the hero banner and the sign-in dialog. Small chips round at 4–5 DIPs and menu rows at 6. Pills (15–16 DIPs) are only for controls laid over art: the back button and the hero status. Circles are reserved for the dial, the avatar, status dots, the 28 DIP icon badges, the selection tick and line radios.

**Brand mark.** Three tapered streaks stacked like light trails, the middle one longest and shifted right, drawn from one geometry: amber on the rail, white on the sign-in art, small and amber perched on empty-state discs. The application icon is the same mark in an amber gradient (#F7CB72 to the accent) on a rounded graphite tile (corner 22% of the tile, #2A2733 to #121116 top to bottom), drawn by `installer/make-brand-assets.ps1` at 16–256 px. The installer's side image is graphite with a warm amber glow, diagonal amber streaks at −22°, the mark and the product name; its small header image is the tile on white. All three assets regenerate from that script rather than being hand-edited.

**Icons.** Outline strokes on a 24 DIP grid at a 1.8 stroke whatever the display size, with round caps and joins; empty-state icons (38–46 DIPs) thin to 1.6. Only the bolt is ever filled (the dial, the cover hover disc, the running badge, the sign-in feature row).

**Cover art.** Each game gets a generated cover: a diagonal gradient in its hue (hues are dealt in catalogue order from a list spaced so neighbours never share a family, skipping the muddy olive band), a soft light pool at upper right, seven light streaks at −22° that fade at both ends, and the game's first character set huge and faint (15% white) bleeding off the bottom-right corner, all inside a 12% white hairline. The hero adds a bottom shade under its white title; the sign-in panel uses a night variant (dark ground, twelve brighter streaks, amber hue).

## Components

### Buttons

Web primary actions use cyan with white text; secondary actions use white with a border. Hover darkens the primary or tints the secondary. Keyboard focus uses a visible outline (3px with 3px offset); disabled buttons use reduced opacity (0.5).

- **Client shape:** gently rounded (8 DIPs), no border.
- **Client primary:** amber fill, near-black brown SemiBold text (14), at least 42 DIPs tall; hover lighter gold, press deeper amber. Disabled turns Raised Graphite with Ash Text rather than fading the amber.
- **Client secondary:** Raised Graphite with Moon Text (13), padding 8 × 14, at least 36 DIPs tall; hover Lift, press Graphite Surface, disabled at 45% opacity. Used for 停止加速, 查看, 清除搜索.
- **Ghost:** transparent, Dusk Text, left-aligned, 6 DIP corners, at least 34 DIPs; hover Lift with Moon Text. Menu rows and 稍后登录. Icon buttons are 38–40 DIP square ghosts.
- **On-art pill:** rail tone at 55%, white text and icon, 15 DIP corners; hover 80%. Only over cover art.
- **Focus:** keyboard only, a 1.5 DIP amber ring at 8 DIP corners drawn 2 DIPs outside the control (round controls get a round ring 5 DIPs out). A mouse click leaves nothing behind.

### Chips

Small filled web badges display operational state text. Green denotes active/succeeded/ready or enabled flags, warm red failure/degradation, and blue work in progress. The badge's label determines what the color claims; an enabled flag alone does not imply a verified connection.

In the client: **推荐** marks the lowest-latency selectable line (Amber Wash, amber SemiBold 11, 4 DIP corners). Cover badges sit at the cover's top corners: **上次加速** on black at 60% with white 11, and **加速中** in green with dark green ink and a filled bolt. Hero region chips pair an amber Bahnschrift code with a white name on a scrim.

### Cards / Containers

Resource tables use white bordered containers with subdued headers, row hover tint and inline actions. Metrics share one segmented container. Empty tables retain their column context and explain loading, no records, or no matching search results. Errors appear in a separate alert with retry; unknown overview counts remain an em dash during loading and after failure.

- **Cover card:** Graphite Surface, 12 DIP corners, 8 DIP padding, no border. The 16:9 cover (8 DIP corners) sits above the name (SemiBold 15) and a pin icon with the region list (label size). Hover raises the card to Raised Graphite and reveals a 42 DIP amber disc with a filled bolt and the Float shadow at the cover's bottom right.
- **Panel:** Graphite Surface at 12 DIP corners for the dial panel, chart and facts strip; tone alone separates it from the page.
- **Stat tile:** Graphite Surface, 12 DIP corners, padding 14 × 16; a 28 DIP round badge in a state tint with its icon (延迟 gauge in green, 抖动 wave in blue, 丢包 packet in orange, 已加速 clock in amber), a meta label, then the figure.

### Inputs / Fields

Web fields use white fill, a cool gray border, and visible focus outline (2px with 1px offset). Search includes a vector magnifier; labels stay visible above editor fields.

Client fields are Raised Graphite with no visible edge at rest, 8 DIP corners, at least 40 DIPs tall, 14 DIP text, amber caret and selection. Hover draws a Strong Seam edge; keyboard focus turns the edge amber. Labels sit above in Dusk Text (12). The library search adds an inset 17 DIP magnifier and the placeholder 搜索游戏 in Ash Text.

### Navigation

Desktop administration navigation stays visible beside the content; selection uses a pale cyan fill, stronger text and a directional icon. Its narrow layout scrolls horizontally.

Client rail items are 60 × 60: a 22 DIP outline icon over an 11 DIP label in Ash Text. Hover fills Lift; selected fills Raised Graphite, turns icon and label amber and shows a 3 × 22 DIP amber bar at the rail edge; disabled drops to 35%. A green 8 DIP dot sits on 加速 while a session runs. The avatar is a 40 DIP circle of Amber Wash with a 35% amber ring (solid amber on hover), showing the account initial in amber Bahnschrift, or an outline user icon when signed out.

### Region Tile and Line Row

- **Region tile:** 128 × 62 DIPs, Graphite Surface, 10 DIP corners: a 36 DIP rounded code badge (Bahnschrift bold 13, white, on a muted fill whose hue is dealt per region so neighbours sit at least 100° apart) beside the name (SemiBold 14). Hover Raised Graphite. Selected: amber 1.5 DIP edge, Amber Wash fill, and an 18 DIP amber tick disc overhanging the top-right corner.
- **Line row:** at least 60 DIPs, Graphite Surface, 10 DIP corners, padding 8 × 16. Left to right: signal bars, the name (SemiBold 14) with an optional 推荐 chip over a 6 DIP load dot and load text, the historical latency figure and its unit, and a radio. Selected: amber edge, Amber Wash fill, amber ring with an amber dot. Unavailable and refused rows are disabled at 45% and cannot be picked. The recommended line is the lowest-latency selectable one and is chosen automatically unless the player's remembered line is still selectable.
- **Signal bars:** four rising rounded bars (22 × 18 DIPs). Lit count follows latency: 4 at 60 ms or less, 3 to 100, 2 to 160, 1 above; lit in the quality color, unlit in Strong Seam; none lit without a measurement.

### Accelerate Dial

The game page's one large control: a 168 DIP round button inside a 6 DIP track ring, with the comet riding the ring and an optional glow behind.

- **Cannot start** (no game, region or selectable line; signed out; network core missing; command pending): Raised Graphite disc, filled bolt and 开始加速 in Ash Text, Seam track, no glow, no comet; disabled.
- **Ready:** amber gradient disc, near-black brown bolt (38) and 开始加速 (SemiBold 18), track at 35% amber, glow 32%, slow gold comet.
- **Connecting or stopping:** Raised Graphite disc with 正在连接 or 正在停止 over the current step in label size (正在分配线路, 正在建立连接, 正在重新连接, 正在回收资源, 正在提交), Seam track, fast comet.
- **Connected:** Raised Graphite disc with the live latency (Bahnschrift 48, quality color) and ms, captioned 当前延迟 (线路波动 when degraded); track and glow in the state color; medium comet.

Under the dial: an 8 DIP state dot and one state sentence (未加速, 正在分配线路, 已连接，可以启动游戏了, 线路波动，正在重新检测, 本机正在为另一个 Windows 用户加速, 本地服务未连接). A failed start or stop appears directly beneath as a centered red line (12.5), not as a toast. When idle, a divider and summary follow (区服; 线路 · 历史 N ms); while running, a full-width secondary 停止加速 button replaces the summary.

### Hero Banner and Dashboard

- **Hero banner:** the game's shaded cover at 14 DIP corners, 184 DIPs high, carrying the on-art back pill (游戏库), the title (Bold 30, white), region chips, and a status pill at bottom right (rail tone at 70%, 16 DIP corners) with the state dot and 未加速, 正在连接, 正在停止, 本地服务未连接 or 加速中 with elapsed time.
- **Latency chart:** 延迟走势 with the provenance caption at right. Four gridlines on a round-number scale (steps of 10, 20, 25, 50, 100 ms and up) with Bahnschrift axis labels; an area falling from the quality color at 35% to transparent under a 2 DIP line; hollow sample dots up to 40 samples; the latest point with a halo and its value in ms; time labels N 秒前 and 最新. It keeps the session's last 120 tunnel probes, plots only measured samples and shows 等待首次检测 before the first.
- **Facts strip:** icon, label and value for 线路, 区服 and 出口 IP (IP in Bahnschrift).

### Empty States and Feedback

- **Empty pane:** centered column at most 440 DIPs wide: a 132 DIP Graphite Surface disc with an inner Seam ring, a 46 DIP outline icon in Dusk Text (plug: service unreachable; user: signed out; alert: catalogue failure or network core missing; refresh: loading; gamepad: no games), and the small amber brand mark perched at the disc's upper right. Then a title (SemiBold 19), an explanation in meta and one primary action only when the player can act (登录账号, 重试). While it shows, rail navigation unchecks and disables.
- **Empty search:** a smaller 104 DIP disc with a magnifier, 没有找到"…", one line of advice and a secondary 清除搜索.
- **Toast:** bottom right, Raised Graphite, 10 DIP corners, Float shadow; a 28 DIP round tinted badge with an info, check, alert or error icon beside 13 DIP text. It leaves on its own: 6 s, errors 20 s.
- **Account menu:** Raised Graphite, 248 DIPs, 10 DIP corners, padding 6, Float shadow. A header pairs a 38 DIP Amber Wash initial disc with the account (SemiBold 14) and server (label); Seam dividers separate ghost rows with 17 DIP icons (切换账号, 退出登录, 复制连接诊断, 退出流光加速器); the version closes it in Bahnschrift label size. Clicking anywhere outside closes it.
- **Sign-in dialog:** the window dims to the rail tone at 88%; a Graphite Surface dialog (14 DIP corners, padding 8, Float shadow) holds two panels. Left, a night-variant amber cover (10 DIP corners) carries the white brand mark, 流光加速器 (Bold 24), a one-line tagline, and three feature rows, each a 30 DIP disc of 20% white with a white icon (grid, signal bars, bolt). Right, a close button, the 登录 headline, labeled fields, a red error line, a primary 登录 (default), a ghost 稍后登录 (cancel) and a centered label-size note.

### Named Rules

**The Probe Label Rule.** Two latencies exist and are never mixed or presented as in-game latency. Line rows show the control plane's historical line probe, captioned above the list "延迟为线路历史检测值", and the idle summary repeats it as "历史 38 ms" after the line name. The dial, the 延迟 tile, the library banner and the chart show the running session's current tunnel probe, captioned on the chart "当前隧道检测，约每 30 秒一次". Missing evidence is an em dash with its unit hidden; packet loss stays an em dash when the probe did not measure UDP. A stopped or new session clears the figures and the chart's history, and losing the local service replaces the pages with the disconnected state so no stale figure stays on screen.

**The Locked Session Rule.** While a session starts, runs or stops, everything that would change it locks: search, region tiles, line rows and refresh. The library stays readable: the running game's cover wears the green 加速中 badge, other covers dim to 55%, and clicking one raises a warning toast ("正在为 … 加速，停止后才能切换游戏"). Locked lists dim in place rather than turning stock white.

**The Refused Line Rule.** When the service refuses a start because of the line (full, withdrawn, node down), that line stays refused until the line list is refreshed (the refresh button beside 选择线路, or the library refresh) or three minutes pass: the row disables with a red load dot and "刚才无法使用，刷新后可重试", its latency becomes an em dash, the best remaining line is selected, and the red line under the dial names both ("… 暂时无法使用（cause），已切换到 …"), or says the region has no other usable line.

### Dialogs and feedback (web)

Web editors use native dialogs, two-column fields where space permits, and a footer with cancel and save. Validation errors stay within the form. Published profile versions, path verification, service availability and live session state remain visible as different statuses.

The reviewed captures in `.impeccable/review/` cover desktop administration, game resources, narrow web layout and overview outage, and the rebuilt client's library, game page, connecting and active sessions, library while running, refused start, account menu, sign-in, empty search, lost service, and minimum and maximized windows. Client figures in them come from a test service. They establish visual and unavailable-state evidence; they do not establish successful real-game acceleration or live node availability.

## Do's and Don'ts

### Do:
- **Do** preserve the light administration and dark native-client surface roles.
- **Do** keep status labels, provenance, empty states, and errors visible beside the affected resource or action.
- **Do** retain horizontally scrollable tables on narrow web screens, and a fixed hero over a scrolling game page at the client's minimum size.
- **Do** use local Chinese-capable fonts: Segoe UI with Microsoft YaHei on the web; Microsoft YaHei UI for client words and Bahnschrift for client figures.
- **Do** color every client latency by one scale (60 ms or less green, to 120 ms orange, above red) and light signal bars at 60, 100 and 160 ms.
- **Do** draw new client icons as 24 px outline geometry at a 1.8 stroke with round caps, and regenerate brand rasters from the three-streak mark in `installer/make-brand-assets.ps1`.
- **Do** give each new game a generated cover from its hue.

### Don't:
- **Don't** replace unknown counts or missing probe evidence with zero or invented healthy measurements.
- **Don't** conflate a published rule, verified path, historical probe, and current connection.
- **Don't** claim tested game compatibility or bundle proprietary game artwork without established evidence or assets.
- **Don't** use amber to report quality or state, or cyan anywhere in the client.
- **Don't** outline resting client surfaces or shadow anything that sits in the page; the Float shadow is for layers above it.
- **Don't** brand with a letter monogram; the brand is the three-streak amber mark, and first characters appear only inside generated covers.
- **Don't** leave a focus ring after a mouse click, or hide it from keyboard focus.
