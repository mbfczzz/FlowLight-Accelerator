---
name: FlowLight Accelerator
description: A light administration framed in graphite and a dark graphite Windows game client, one product lit by one amber accent.
colors:
  admin-background: "#f4f3f7"
  admin-surface: "#ffffff"
  admin-surface-2: "#faf9fc"
  admin-line: "#e6e3ec"
  admin-line-strong: "#d4d0dc"
  admin-text: "#1b1a21"
  admin-text-secondary: "#5c5668"
  admin-text-tertiary: "#726c7e"
  admin-sidebar: "#131217"
  admin-sidebar-hover: "#1d1c23"
  admin-sidebar-active: "#26242e"
  admin-sidebar-text: "#b3adbe"
  admin-sidebar-muted: "#7f798b"
  admin-accent: "#e6ae45"
  admin-accent-hover: "#f0bd5c"
  admin-on-accent: "#1c1300"
  admin-accent-text: "#9a640b"
  admin-accent-soft: "#fbf1dc"
  admin-good: "#1d7f4f"
  admin-good-soft: "#e2f3e9"
  admin-warn: "#a95c0c"
  admin-warn-soft: "#fcefdc"
  admin-bad: "#b83a45"
  admin-bad-soft: "#fbe7e9"
  admin-info: "#2c5ea8"
  admin-info-soft: "#e5edfa"
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
  admin-headline:
    fontFamily: '"Microsoft YaHei UI", "PingFang SC", "Segoe UI", system-ui, sans-serif'
    fontSize: "22px"
    fontWeight: 700
  admin-title:
    fontFamily: '"Microsoft YaHei UI", "PingFang SC", "Segoe UI", system-ui, sans-serif'
    fontSize: "15.5px"
    fontWeight: 700
  admin-body:
    fontFamily: '"Microsoft YaHei UI", "PingFang SC", "Segoe UI", system-ui, sans-serif'
    fontSize: "13px"
  admin-label:
    fontFamily: '"Microsoft YaHei UI", "PingFang SC", "Segoe UI", system-ui, sans-serif'
    fontSize: "13px"
    fontWeight: 500
  admin-meta:
    fontFamily: '"Microsoft YaHei UI", "PingFang SC", "Segoe UI", system-ui, sans-serif'
    fontSize: "12px"
  admin-figure-display:
    fontFamily: '"Bahnschrift", "DIN Alternate", "Segoe UI", system-ui, sans-serif'
    fontSize: "32px"
    fontWeight: 500
  admin-figure:
    fontFamily: '"Bahnschrift", "DIN Alternate", "Segoe UI", system-ui, sans-serif'
    fontSize: "14px"
  admin-mono:
    fontFamily: '"Cascadia Mono", "Consolas", ui-monospace, monospace'
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
  tag: "6px"
  control: "8px"
  choice: "9px"
  container: "10px"
  icon-tile: "12px"
  dialog: "14px"
  pill: "999px"
  client-control: "8px"
  client-choice: "10px"
  client-panel: "12px"
  client-hero: "14px"
spacing:
  label-gap: "7px"
  compact: "8px"
  control-gap: "10px"
  tile-gap: "14px"
  field-gap: "16px"
  page-head-gap: "22px"
  drawer: "26px"
  desktop-content: "36px"
  mobile-content: "16px"
  client-row-gap: "8px"
  client-tile-gap: "10px"
  client-card-gap: "16px"
  client-gutter: "20px"
  client-page-game: "24px"
  client-page: "32px"
components:
  button-primary:
    backgroundColor: "{colors.admin-accent}"
    textColor: "{colors.admin-on-accent}"
    rounded: "{rounded.control}"
    padding: "0 14px"
    height: "36px"
  button-primary-hover:
    backgroundColor: "{colors.admin-accent-hover}"
  button-secondary:
    backgroundColor: "{colors.admin-surface}"
    textColor: "{colors.admin-text}"
    rounded: "{rounded.control}"
    padding: "0 14px"
    height: "36px"
  button-danger:
    backgroundColor: "{colors.admin-bad}"
    textColor: "{colors.admin-surface}"
    rounded: "{rounded.control}"
    padding: "0 14px"
    height: "36px"
  input:
    backgroundColor: "{colors.admin-surface}"
    textColor: "{colors.admin-text}"
    rounded: "{rounded.control}"
    padding: "9px 12px"
  navigation-item:
    textColor: "{colors.admin-sidebar-text}"
    rounded: "{rounded.control}"
    padding: "9px 12px"
  navigation-item-active:
    backgroundColor: "{colors.admin-sidebar-active}"
    textColor: "{colors.admin-surface}"
  page-icon:
    backgroundColor: "{colors.admin-sidebar}"
    textColor: "{colors.admin-accent}"
    rounded: "{rounded.icon-tile}"
    size: "44px"
  metric-tile:
    backgroundColor: "{colors.admin-surface}"
    rounded: "{rounded.container}"
    padding: "18px 18px 16px"
  pill-good:
    backgroundColor: "{colors.admin-good-soft}"
    textColor: "{colors.admin-good}"
    rounded: "{rounded.pill}"
    padding: "3px 9px 3px 8px"
  resource-table:
    backgroundColor: "{colors.admin-surface}"
    rounded: "{rounded.container}"
  row-action-emphasis:
    textColor: "{colors.admin-accent-text}"
    padding: "0 9px"
    height: "30px"
  row-action-emphasis-hover:
    backgroundColor: "{colors.admin-accent-soft}"
  choice-selected:
    backgroundColor: "{colors.admin-accent-soft}"
    textColor: "{colors.admin-accent-text}"
    rounded: "{rounded.choice}"
  drawer:
    backgroundColor: "{colors.admin-surface}"
    rounded: "{rounded.dialog}"
    width: "600px"
  confirm-dialog:
    backgroundColor: "{colors.admin-surface}"
    rounded: "{rounded.dialog}"
    width: "440px"
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

FlowLight has two surfaces with one job each, and they read as one product. Operators work in a light administration framed by a graphite sidebar: an overview that says what is ready and what is missing, lists whose first column shows the thing itself (a game's cover, a player's initial, a line's icon) with its state in a dotted pill, and a side drawer for editing so the list stays in view. Players use a dark native Windows client that sits beside a running game, usually in the evening: a game library rather than a settings form. A cover opens each game (the picture the operator uploaded, or generated art), region and line choices are tiles and rows that carry their own evidence, and one round dial starts acceleration and then becomes the live latency readout.

The client's material is warm graphite with a faint violet cast, separated by tone rather than outlines and lit by a single amber, the "flowing light" of the product's name. The administration borrows that material instead of inventing its own: its sidebar is the client's window graphite, its work area a lilac-tinted white whose surfaces lift on a soft shadow rather than an outline, and the same amber marks its brand, actions, selection and focus. On both surfaces latency, health and publication state speak in a separate green, orange, red and blue set. Pictures carry recognition: cover art drawn per game from a hue and a few light streaks (the same hue for the same game on both surfaces), outline icons on one 24 px grid, four signal bars, a latency chart. No game artwork is bundled; an operator may upload a licensed picture per game, which replaces the generated cover on both surfaces. No figure appears without saying what it measures.

**Key Characteristics:**
- One product, one amber: a light administration in a graphite frame and a dark graphite client; amber is brand, action, selection and focus on both, never state.
- Administration: 248 px graphite sidebar grouped 配置 / 账号 / 运行, an overview of four metric tiles and a 上线准备 checklist, card lists with entity first columns and row actions, side-drawer editors, and phone layouts down to stacked row cards.
- Client: 80 DIP icon rail, a library of 16:9 cover cards, a game page with a hero banner, region tiles, line rows and the dial.
- A game's generated cover takes the same hue in both apps, from a cover slot fixed when the game is first stored.
- Every client figure is a probe with a visible provenance label; missing evidence is an em dash.
- Unknown, draft, unavailable, refused, historical and current states remain explicit.

## Colors

The palette splits by surface but shares its light: the administration is a lilac-tinted white work area inside a graphite sidebar; the client is a five-step graphite ladder. Both carry one warm amber and the same four state meanings, tuned as dark inks on pale washes for white and as bright hues for graphite.

### Primary

- **Flowing-Light Amber** (client-accent): the client's only accent. It fills the brand mark, the ready dial (as a gradient from light gold #F7CB72 through the accent to burnt amber #CC8C2B), and primary buttons; it marks the selected rail item (icon, label and a 3 DIP edge bar), the selected region tile and line row (1.5 DIP edge plus tick or radio dot), section-title icons, region codes on the hero, the 推荐 chip, the avatar, the text caret and every keyboard focus ring. Hover lifts it to a lighter gold (client-accent-hover); press deepens it (client-accent-pressed). Text on amber is near-black brown (client-on-accent), never white.
- **Amber Wash** (client-accent-soft): a solid dark amber-brown that fills selected tiles and rows, the avatar disc and the 推荐 chip. The ready dial's track and the avatar ring use the accent at 35% alpha.
- **Flowing-Light Amber on the web** (admin-accent, admin-accent-hover, admin-on-accent): the client's amber, hover gold and near-black ink, unchanged. It fills primary buttons (ink text, never white), the switch when on, the 上线准备 progress bar, the brand mark's streaks and the icon on each page head's graphite tile; it marks the active navigation item (its icon and a 3 px bar on the sidebar's edge), a focused field (amber edge and a 3 px halo at 22%) and the 2 px focus outline.
- **Amber Ink** (admin-accent-text): amber that must read as text or line art on white: the emphasised row action, drawer section icons, the text caret, and the label of a selected toggle or a hovered suggestion.
- **Amber Cream** (admin-accent-soft): the fill under a selected toggle chip or radio card (both also take an amber edge) and the hover of the emphasised row action. Text selection is a light gold (#f6dda8).

### Secondary

Client state colors. Each also has a 14% alpha tint used only as the round badge behind an icon (dashboard tiles, toasts).

- **Signal Green** (client-good): latency of 60 ms or less, load under 50%, an active connection, the 加速中 badge on a cover, the rail dot and taskbar overlay while a session runs.
- **Ember Orange** (client-warn): latency of 61 to 120 ms, load of 50 to 80%, preparing, connecting, reconnecting, degraded and stopping; the packet-loss tile icon.
- **Brake Red** (client-bad): latency above 120 ms, load of 80% or more, refused lines, failed or blocked sessions, inline and sign-in errors.
- **Probe Blue** (client-info): the jitter tile and informational toasts.

Administration state colors are the same meanings as dark inks on pale washes of their own hue (admin-good on admin-good-soft, and so on). A pill pairs the ink with a 6 px dot of the same colour and always a word.

- **Pine Green** (admin-good): 启用, 已发布, 已验证上线, 接收新会话, active sessions, succeeded jobs, a node heartbeat within 60 s, a done checklist step, an overview tile whose count is above zero, and the notice banner after an action.
- **Rust** (admin-warn): 草稿, 验证中 and 新版本验证中, 排空中, preparing, connecting, reconnecting and stopping sessions, pending and running jobs, a heartbeat within 5 minutes, and an overview tile whose count is zero while what it depends on exists.
- **Brick Red** (admin-bad): 验证未通过 and 新版本未通过 (with the reason printed beneath), failed jobs, degraded sessions, page and form alerts, field errors, 停止 and 终止线路, and the danger confirm button.
- **Cobalt** (admin-info): reserved for informational states; nothing in the administration uses it at present.

Grey (Slate text on the lilac tint #efedf3) is the administration's default: 停用, 未发布, 未验证, 未上线, 从未上线, a stale heartbeat, a tile while its count is unknown, and plain category pills (mode, endpoint and job kind, role, release channel), which drop the dot.

### Neutral

The administration's frame is **Sidebar Graphite** (admin-sidebar, the client's Night Graphite): the sidebar, the page-head icon tile and the base of the login brand panel. Navigation items lift to **Sidebar Hover** (admin-sidebar-hover) and, when active, **Sidebar Active** (admin-sidebar-active) with white text; labels are **Sidebar Text** (admin-sidebar-text, the client's Dusk Text), and group names, icons and the FLOWLIGHT ADMIN line are **Sidebar Muted** (admin-sidebar-muted). The work area is **Mist** (admin-background). **Card White** (admin-surface) carries tiles, the checklist, list cards, drawers, dialogs, menus and fields; **Paper** (admin-surface-2) carries table heads, the drawer footer, hints, read-only fields and code blocks. **Hairline** (admin-line) divides rows, steps and drawer sections; **Strong Hairline** (admin-line-strong) edges fields, secondary buttons and unselected toggles and radio cards. **Ink** (admin-text, the client's Graphite Surface) is primary text; **Slate** (admin-text-secondary) carries descriptions, field labels, ghost and row actions; **Haze** (admin-text-tertiary) carries table heads, sub lines, notes and counts. A lilac tint (#efedf3; #f1eff5 under chips, tags and menu hover) fills ghost hover, grey pills, entity icon tiles, the progress track and step numbers.

The client's neutrals form a tone ladder, darkest to lightest: **Rail Black** (client-sidebar) for the icon rail, **Night Graphite** (client-background) for the window, **Graphite Surface** (client-surface) for cards, panels, tiles and rows, **Raised Graphite** (client-elevated) for hover, secondary buttons, fields, the idle dial, toasts and the account menu, and **Lift** (client-hover) for hover on raised items. **Seam** (client-border) draws dividers, the idle dial track and chart gridlines; **Strong Seam** (client-border-strong) draws unselected radio rings, field hover edges, unlit signal bars and scrollbar thumbs. **Moon Text** (client-text) is primary ink; **Dusk Text** (client-text-secondary) carries meta lines and labels; **Ash Text** (client-text-tertiary) carries captions, units, unknown states and disabled ink.

Scrims are the rail tone at partial alpha: 55% under the back pill on art, 70% under the hero status pill, 88% behind the sign-in dialog; cover badges sit on black at 60%. The administration's dialogs and drawers sit over its sidebar graphite at 45%.

### Named Rules

**The One Amber Rule.** Amber marks the brand, the action, the selection and keyboard focus, on both surfaces, and nothing else. It never reports quality or state: latency, load, connection, publication and health speak only in green, orange, red and blue.

**The State-Tone Rule.** In the administration a tone is a reading, never decoration. A pill shows a dot and a word, so colour is never the only signal; an overview tile's icon badge is green when its count is above zero, orange when it is zero but what it depends on exists (节点 added, none online), and grey while the count is unknown or there is nothing to have yet. Identity is not state: an avatar takes a soft colour of its own from the cover palette (its hue picked from the name, a 92% lightness wash under a 32% initial), roles and release channels are plain grey pills without a dot, and the line editor's hop labels (入口, 中转, 落地) are grey.

**The Evidence Rule.** A color, bar count or figure must describe reported state: an enabled resource, a published profile, a verified path, a line's historical probe and a live tunnel probe are separate facts. A line without a measurement shows no lit bars, a tertiary em dash and no unit; a line with unknown capacity says 负载未知 in tertiary rather than guessing a load color.

**The Shared Cover Rule.** A game wears one colour everywhere, and adding a game never recolours another. When a game is first stored, the control service gives it a cover slot one above the highest so far, inside the save transaction; edits keep the slot, and games stored before slots existed (and the built-in drafts) are backfilled in id order when the store opens and after seeding. The service sends cover_index (the slot minus one) to the administration's lists and the client's catalogue alike, and the admin's `Cover` and the client's `Palette.SteppedHue` both take the hue at that index in one nine-hue list (222, 340, 168, 22, 252, 140, 312, 192, 6) whose neighbours sit far apart. Only a game without an index falls back to a hash of its id. Region code badges are coloured by position in their own list (hues 205, 345, 155, 25, 270, 120 at 45% saturation and 42% lightness), so neighbours sit at least 100° apart in both apps; in the editor's suggestions a region not yet chosen is grey.

## Typography

**Admin Interface Font:** Microsoft YaHei UI (with PingFang SC, Segoe UI, system-ui)
**Admin Figure Font:** Bahnschrift (with DIN Alternate, Segoe UI)
**Admin Identifier Font:** Cascadia Mono (with Consolas, ui-monospace)
**Client Interface Font:** Microsoft YaHei UI (with Segoe UI, Microsoft YaHei)
**Client Number Font:** Bahnschrift (with Segoe UI Semibold, Microsoft YaHei UI)

**Character:** All faces are local system faces; no font files are shipped or requested. YaHei UI keeps Chinese and Latin labels even at small sizes, and Bahnschrift's narrow DIN-like figures give every measurement an instrument read and keep columns of milliseconds compact. The administration adds a monospace only for machine strings: ids, hosts, addresses, hashes and enrollment secrets.

The administration's root size is 14px. Page titles are 22px Bold beside the 44px graphite icon tile, over a 13px Slate description; drawer titles are 18px, confirm titles 17px, drawer section titles and the checklist heading 15.5–16px Bold. Tables, buttons and descriptions run at 13px (entity names 13.5px SemiBold), fields and navigation at 13.5px; field labels are 13px Medium in Slate; table heads, sub lines, notes and pills are 12–12.5px. Overview figures are Bahnschrift 32px Medium with tabular numerals (26px at 560px and below); counts, ports, capacity, attempts, versions, step numbers and avatar initials are Bahnschrift 14px. The login brand panel's headline is 40px Bold and the form's title 26px. Hints and alerts use a 1.6 line height.

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

**The Instrument Figures Rule.** Every measured or coded value (milliseconds, percent, elapsed time, exit IP, region codes, the avatar initial, the version string) is set in Bahnschrift, and words never are. The unit follows the figure at label size in Ash Text, bottom-aligned, and disappears with the figure when there is no measurement. The administration follows the same rule for its counts and codes, and sets ids and hosts in Cascadia Mono.

## Layout

The administration is a fixed sidebar (248px, sticky, full height) beside a content column capped at (1440px) and padded (30px 36px 48px). The sidebar stacks the brand (the mark over 流光加速器 and FLOWLIGHT ADMIN), 运行总览 on its own, then 配置 (游戏与规则, 加速线路, 节点服务器, 服务端点, 落地绑定), 账号 (用户与权限, 连接设备) and 运行 (加速会话, 部署任务, 客户端版本, 操作审计) under small muted labels, and ends with the account: avatar, name, role and a 退出登录 icon button below a divider. Every page opens with a page head: a 44px graphite tile with the section icon in amber, the title and one line of description, and at right 刷新 (secondary) and 新增 (primary, when the role may edit); alerts and notices stack beneath it.

The overview is four metric tiles in a row ((14px) gaps) above the 上线准备 checklist panel. A list is a toolbar (a (320px) search, and at right a count that claims a total only once the last page has loaded), the list card, and, while more pages exist, a footer with 加载更多.

Width steps down four times. At (1100px) or less the tiles go two-up. At (860px) or less the sidebar becomes a (56px) top bar: the brand, a divider and the current section's name, and a (40px) menu button at right that opens the whole navigation as a full-height graphite sheet under the bar. Content padding becomes (22px 16px 40px), the page head stacks its actions under the title, search spans the width, drawer fields become one column, the drawer goes full width with square corners and (18px) side padding, and login shows the form alone. At (720px) or less each table row becomes its own card: the entity cell across the top, every other cell a label-value line named by its column (data-label, an (84px) label column), and the row actions wrapping left-aligned at the bottom; nothing scrolls sideways. At (560px) or less the tiles stay two-up but tighten (padding (14px), figures (26px), notes hidden), checklist actions drop under their text and detail labels narrow to (92px). Dialogs keep (16px) side clearance and at most (88vh).

The client window opens at (1120 × 750 DIPs) with a minimum of (900 × 650 DIPs). An icon rail (80 DIPs) on Rail Black holds the brand mark at top, two navigation items (60 × 60 DIPs; 加速 is enabled only once a game is chosen) and the avatar (40 DIPs) at the bottom. Content to its right is capped at (1320 DIPs) and centered, so a maximized window grows margins rather than stretching rows.

The library has page margins of (32 DIPs). Its header row carries the page title, a search field (260 DIPs) and a refresh icon button. Cards flow in as many columns of about (236 DIPs) as fit, with (16 DIP) gaps; each cover is 16:9 of the card's inner width. While a session runs, a banner panel sits above the grid with a small cover (112 × 63 DIPs), a green dot, 正在加速 and the game, region · line, the live latency and a 查看 button.

The game page has margins of (24 DIPs). A hero banner (184 DIPs high) spans the width and stays fixed; everything below scrolls vertically. Below it sits a flexible choices column, a (20 DIP) gutter and a fixed dial panel (316 DIPs) aligned to the top. The choices column stacks wrapping region tiles ((10 DIP) gaps) above line rows ((8 DIP) gaps). While a session runs or stops, the choices column is replaced by the dashboard: four stat tiles (two columns when the column is narrower than 540 DIPs), the latency chart panel (chart 170 DIPs high) and a wrapping facts strip. At the minimum window size the dial panel stays beside the choices and the page scrolls; it is a native minimum-size layout, not a mobile breakpoint.

Overlays anchor to the window: the toast at bottom right (28 and 24 DIPs in, at most 460 DIPs wide), the account menu rising from beside the avatar (248 DIPs wide), and the sign-in dialog centered (740 × 450 DIPs, a 310 DIP art panel beside the form).

## Elevation & Depth

The administration separates surfaces by a soft shadow on Mist, not by outlines. Hairlines live only inside a surface: under table heads and between rows, between drawer sections, steps and detail rows, and above the drawer footer. Feedback eases colour, border and shadow over 0.15s; the drawer slides 32px in from the right over 0.22s, the 更多 menu drops 4px in over 0.12s and the phone menu sheet 8px over 0.16s. Reduced motion removes every web animation and transition.

The client is tonal. The five-step ladder from Rail Black to Lift separates rail, page, surfaces and interactive lift, and resting surfaces carry no outline. One shadow exists, reserved for layers that float above the page, and the dial adds light instead of shadow.

### Shadow Vocabulary

- **Rest** (web, `0 1px 2px rgb(27 26 33 / 4%), 0 4px 16px rgb(27 26 33 / 5%)`): metric tiles, the checklist panel, list cards and, on phones, each row card and the empty-state card.
- **Menu** (web, `0 12px 32px rgb(19 18 23 / 16%), 0 2px 6px rgb(19 18 23 / 6%)`, with a Hairline edge): the 更多 menu.
- **Overlay** (web, `0 18px 50px rgb(19 18 23 / 22%)` over a `rgb(19 18 23 / 45%)` backdrop): drawers and dialogs.
- **Float** (WPF `DropShadowEffect` BlurRadius 28, ShadowDepth 8, Direction 270, Opacity 0.5, black; about `0 8px 28px rgb(0 0 0 / 50%)`): the toast, the account menu, the sign-in dialog, and the amber bolt disc that appears on a cover under hover.
- **Dial glow** (an amber ellipse behind the dial under a 44 DIP blur): 32% opacity when ready; in the state color at 22% while connected; off otherwise.

### Motion

The dial ring carries a comet: an 80° arc fading from transparent to bright, rotating continuously. Its speed and color encode state: one turn per 6 s in light gold when ready, 1.1 s in amber while connecting (1.1 s in Ash Text while stopping), 3.2 s in green (orange when degraded) while connected; no comet when the dial cannot start. When Windows client-area animation is off, the arc holds still. All other feedback is an immediate tone change with no transition.

### Named Rules

**The Tone-Not-Line Rule.** Surfaces separate by tone, not outline. An edge appears only to say something: an amber 1.5 DIP edge for selection, a field's hover or focus edge, a 12% white hairline around cover art. The administration keeps the rule with shadow in place of tone: no resting surface is outlined, and edges belong to controls (fields, secondary buttons, choices, switch rows) and to dividers inside a surface.

## Shapes

The administration uses a matching ladder: 4px for region code badges, 6px for chips and tags, 8px for buttons, fields and icon buttons (7px for small buttons, row actions, menu rows and covers), 9px for alerts, switch rows, radio cards, the segmented control, version rows and entity icon tiles, 10px for tiles, panels, list cards and the menu, 12px for the page-head icon tile, and 14px for dialogs and the drawer's left corners. Pills, toggle chips, suggestion chips and the switch are fully round; circles are kept for avatars (34px, 20px small), tile icon badges (34px), checklist step marks (32px), confirm icons (40px), empty-state discs (64px) and pill dots. Web icons are Lucide outlines at their 2 px stroke, 14–26px.

The client uses a soft four-step corner ladder: controls (8 DIPs) for buttons, fields, the cover inside a card, region code badges and focus rings; choices and transient layers (10 DIPs) for region tiles, line rows, the toast and the account menu; panels and cards (12 DIPs) for library cards, the dial panel, stat tiles, the chart panel and the rail item highlight; the hero (14 DIPs) for the hero banner and the sign-in dialog. Small chips round at 4–5 DIPs and menu rows at 6. Pills (15–16 DIPs) are only for controls laid over art: the back button and the hero status. Circles are reserved for the dial, the avatar, status dots, the 28 DIP icon badges, the selection tick and line radios.

**Brand mark.** Three tapered streaks stacked like light trails, the middle one longest and shifted right, drawn from one geometry: amber on the rail, white on the sign-in art, small and amber perched on empty-state discs. The application icon is the same mark in an amber gradient (#F7CB72 to the accent) on a rounded graphite tile (corner 22% of the tile, #2A2733 to #121116 top to bottom), drawn by `installer/make-brand-assets.ps1` at 16–256 px. The installer's side image is graphite with a warm amber glow, diagonal amber streaks at −22°, the mark and the product name; its small header image is the tile on white. All three assets regenerate from that script rather than being hand-edited. The administration draws the same tile and gradient as SVG (`BrandMark`, 36 px in the sidebar, 40 px on the login panel) and a flat version as its favicon.

**Icons.** Outline strokes on a 24 DIP grid at a 1.8 stroke whatever the display size, with round caps and joins; empty-state icons (38–46 DIPs) thin to 1.6. Only the bolt is ever filled (the dial, the cover hover disc, the running badge, the sign-in feature row).

**Cover art.** Each game gets a generated cover: a diagonal gradient in its hue (the hue at the game's cover_index in a list spaced so neighbours never share a family, skipping the muddy olive band; see The Shared Cover Rule), a soft light pool at upper right, seven light streaks at −22° that fade at both ends, and the game's first character set huge and faint (15% white) bleeding off the bottom-right corner, all inside a 12% white hairline. The hero adds a bottom shade under its white title; the sign-in panel uses a night variant (dark ground, twelve brighter streaks, amber hue). The administration draws a small version of the same cover at 64 × 36 in the games list (44 × 25 in session rows, 22 × 13 in pickers): the hue gradient and light pool, three streaks at −22° tinted toward the hue, and the faint first character.

**Uploaded covers.** An operator can upload a picture per game in the game drawer's 封面 section (JPG, PNG or WebP, at least 640 × 360, at most 10 MB). The control service crops the largest 16:9 area around the centre, scales it to 1280 × 720 at most and re-encodes it as JPEG, stores it under the SHA-256 of that JPEG and serves it without a token at `/api/client/v1/covers/<sha256>` with a year-long immutable cache. The client checks each download against its name, keeps it on disk, and draws it filling the cover, centred across and 35% down, so faces in key art stay in the wide game page banner; the bottom shade and hairline stay on top. Anything that cannot be loaded leaves the generated cover. The drawer previews the picture on the client's dark ground as a library card and as the banner, before it is saved; saving the game applies it, and no publication is needed.

## Components

### Buttons

Web buttons are 36px tall with 8px corners and 13px SemiBold text, icon before label (30px and 7px corners in the small size). **Primary** is amber with near-black ink, lifting to the hover gold; one per view (新增, 保存, 登录, a confirm). **Secondary** is white with a Strong Hairline edge and a Paper hover (刷新, 取消, 加载更多, an undone checklist step's action). **Danger** is Brick Red with white text, used only to confirm a risky action. **Ghost** is Slate text with a lilac hover (a done step's 查看, 添加一跳); icon buttons are 34px ghost squares. Disabled buttons drop to 50% opacity; keyboard focus draws a 2px amber outline 2px out, and a mouse click leaves none.

- **Client shape:** gently rounded (8 DIPs), no border.
- **Client primary:** amber fill, near-black brown SemiBold text (14), at least 42 DIPs tall; hover lighter gold, press deeper amber. Disabled turns Raised Graphite with Ash Text rather than fading the amber.
- **Client secondary:** Raised Graphite with Moon Text (13), padding 8 × 14, at least 36 DIPs tall; hover Lift, press Graphite Surface, disabled at 45% opacity. Used for 停止加速, 查看, 清除搜索.
- **Ghost:** transparent, Dusk Text, left-aligned, 6 DIP corners, at least 34 DIPs; hover Lift with Moon Text. Menu rows and 稍后登录. Icon buttons are 38–40 DIP square ghosts.
- **On-art pill:** rail tone at 55%, white text and icon, 15 DIP corners; hover 80%. Only over cover art.
- **Focus:** keyboard only, a 1.5 DIP amber ring at 8 DIP corners drawn 2 DIPs outside the control (round controls get a round ring 5 DIPs out). A mouse click leaves nothing behind.

### Chips

Web state is a **pill**: fully round, 12px SemiBold, a 6px dot in the ink colour before the word, on the state's wash. Categories use a plain grey pill without the dot (平台托管, endpoint and job kinds). A line's 发布与验证 cell can hold two pills, the serving version (已验证上线) beside a newer one (新版本验证中 or 新版本未通过), from the latest_status and latest_error the control service attaches (`annotateLines`); when the newest version failed, its reason follows on the next line in 12px Brick Red, cut with an ellipsis and whole on hover. A region is drawn by one `RegionName` everywhere (list chips, a node's region, detail drawers, editor tags and suggestions): a 4px code badge (Bahnschrift 10.5, white on its position colour, Sidebar Muted grey for an unchosen suggestion) and the region's Chinese name when known. In lists it sits in a **chip** with 6px corners on the lilac tint; an empty list says 待配置. Times are relative (刚刚, N 分钟前, N 小时前, N 天前; 分钟后 and 小时后 for leases) with the exact time in the tooltip, and an em dash when there is none.

In the client: **推荐** marks the lowest-latency selectable line (Amber Wash, amber SemiBold 11, 4 DIP corners). Cover badges sit at the cover's top corners: **上次加速** on black at 60% with white 11, and **加速中** in green with dark green ink and a filled bolt. Hero region chips pair an amber Bahnschrift code with a white name on a scrim.

### Cards / Containers

- **Metric tile (web):** Card White, 10px corners, Rest shadow; a 34px round icon badge in the tile's state tone beside the label, the Bahnschrift figure, and a note that names the whole when known (共 3 个节点 · 45 秒内有心跳). The four are 当前会话, 在线节点, 已验证线路版本 and 已发布游戏; a count stays an em dash while loading and after a failure.
- **上线准备 checklist (web):** a panel with the heading, 已完成 N / 5 (an em dash until anything is known), a 6px amber progress bar, and five steps in setup order: 发布游戏规则, 接入节点服务器, 配置服务端点, 发布并验证线路, 开通玩家账号. Each step is a 32px numbered disc, a title, a sentence with live counts, and a small button to the section: secondary with its action while undone; ghost 查看 once done, when the disc turns green with a tick.
- **List card (web):** a white card with the Rest shadow, a sticky Paper head (12px Haze), 13px cells, hairline rows and a warm tint on hover. The first column is the **entity**: a lead picture (the game's cover, an avatar initial, or a 34px lilac icon tile for lines, nodes, endpoints and devices), the name in SemiBold and a sub line (a game's processes or 未填写游戏进程, a line's hops and priority, a node's Agent version or 尚未注册, a user's role note). References show names, with the id in the tooltip.
- **Empty list (web):** inside the card, a 64px lilac disc with an icon, a title and one sentence, telling loading (正在读取…), a failed read (未能读取记录, never called empty), no match (没有匹配的记录, saying when only loaded rows were searched) and a true empty list (还没有记录, what the list is for, and 新增 when the role may create).
- **Cover card:** Graphite Surface, 12 DIP corners, 8 DIP padding, no border. The 16:9 cover (8 DIP corners) sits above the name (SemiBold 15) and a pin icon with the region list (label size). Hover raises the card to Raised Graphite and reveals a 42 DIP amber disc with a filled bolt and the Float shadow at the cover's bottom right.
- **Panel:** Graphite Surface at 12 DIP corners for the dial panel, chart and facts strip; tone alone separates it from the page.
- **Stat tile:** Graphite Surface, 12 DIP corners, padding 14 × 16; a 28 DIP round badge in a state tint with its icon (延迟 gauge in green, 抖动 wave in blue, 丢包 packet in orange, 已加速 clock in amber), a meta label, then the figure.

### Row Actions (web)

The last column holds the row's actions, right-aligned: 30px tall, 12.5px Slate with an icon, lilac on hover.

- **Primary action** first, in Amber Ink with an Amber Cream hover, while the row still needs it (发布规则 for a draft game, 发布 for an unpublished line, 注册凭据 for a node with no Agent); once done it stays as a plain action.
- **编辑** when the role may edit.
- **连通性** (lines), with the Activity icon, for every role: opens the connectivity check in the right-hand drawer. Below 1500px the label folds into the icon, which keeps 检查连通性 as its tooltip and accessible name, so the row still fits.
- **更多**: a menu below the button, or above it when there is no room, with 36px icon rows. On lines: 排空 or 恢复接收新会话, 查看版本与验证记录, and last, after one hairline, 终止线路 and 删除 in Brick Red with a red wash on hover. Games, nodes, endpoints, bindings and releases carry it with 删除 alone, for the roles that may delete them (lines and games take the publisher's right, as publishing them does). Arrow keys move through it; Escape closes it and returns focus to the button.
- **详情** last, as an icon-only eye.

A destructive action that stands in the row (停止 on a live session) is Brick Red text with a red wash on hover. Every state change goes through a confirmation; while one runs for a row, all its actions disable and the running one reads 处理中….

### Inputs / Fields

Web fields are Card White with a Strong Hairline edge, 8px corners, padding 9 × 12, 13.5px text and an Amber Ink caret; hover darkens the edge, focus turns it amber with a 3px amber halo at 22%. Labels sit above in 13px Medium Slate, notes below in 12px Haze. The list search adds an inset magnifier. Composite controls share the look: **SwitchRow** (a bordered row with title, note and a 40 × 22 switch, amber when on), **Segmented** (a lilac track whose chosen option turns white with a small shadow), **Toggles** (round chips; selected takes an amber edge on Amber Cream), **RadioCards** (choices with a title and a line of explanation; selected takes an amber edge, an inner amber ring and Amber Cream), **TagInput** (chips added by Enter, comma, paste or leaving the box, with dashed suggestion chips that turn amber on hover) and **PickList** (the same, choosing rows by name with their cover or avatar).

An invalid field turns its label and edge Brick Red (a red halo at 18% on focus) and shows its message in place of its note; the drawer scrolls it to the middle and puts the cursor in it.

Client fields are Raised Graphite with no visible edge at rest, 8 DIP corners, at least 40 DIPs tall, 14 DIP text, amber caret and selection. Hover draws a Strong Seam edge; keyboard focus turns the edge amber. Labels sit above in Dusk Text (12). The library search adds an inset 17 DIP magnifier and the placeholder 搜索游戏 in Ash Text.

### Navigation

Administration navigation is the graphite sidebar: 13.5px items with 18px icons, padding 9 × 12. Hover lifts an item to Sidebar Hover with white text; the active item fills Sidebar Active with white SemiBold text, turns its icon amber and shows a 3px amber bar at the sidebar's edge. On phones the same list is the top bar's menu sheet at 14.5px, and choosing an item closes it.

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
- **Account menu:** Raised Graphite, 248 DIPs, 10 DIP corners, padding 6, Float shadow. A header pairs a 38 DIP Amber Wash initial disc with the account (SemiBold 14); the server address is never shown to players. Seam dividers separate ghost rows with 17 DIP icons (切换账号, 退出登录, 复制连接诊断, 检查更新, 退出流光加速器); the version closes it in Bahnschrift label size. Clicking anywhere outside closes it.
- **Sign-in dialog:** the window dims to the rail tone at 88%; a Graphite Surface dialog (14 DIP corners, padding 8, Float shadow) holds two panels. Left, a night-variant amber cover (10 DIP corners) carries the white brand mark, 流光加速器 (Bold 24), a one-line tagline, and three feature rows, each a 30 DIP disc of 20% white with a white icon (grid, signal bars, bolt). Right, a close button, the 登录 headline, labeled fields, a red error line, a primary 登录 (default), a ghost 稍后登录 (cancel) and a centered label-size note.

### Named Rules

**The Coexistence Rule.** Another proxy on the machine keeps everything that is not the game's: when its SOCKS port answers (only listening ports are tried, so a machine without one pays nothing), the kernel hands it that traffic (UPSTREAM), with that proxy's own process sent DIRECT; a proxy whose process cannot be named is not used. Domain names survive the hand-off, so the other proxy's own rules keep working. Its TUN mode must be off: two TUNs claim the default route at the same metric, so which one carries the game is Windows' choice, and the game's lookups may be answered with the other proxy's fake addresses, which only it can map back. v0.1.12 ran alongside an active TUN and its tunnel probe failed every time; from v0.1.13 the start is refused with the software named, saying that its port will carry the rest once the TUN is off. Our fake-IP range is 198.19.0.0/16, the upper half of the benchmarking block 198.18.0.0/15, away from the 198.18.0.1/16 Clash-family kernels answer from by default. sing-box defaults to the whole /15 but allocates from the bottom, reaching 198.19 only after 65,000 names.

**The Bound-Before-Probe Rule.** mihomo opens its listeners before its TUN, and only with the TUN does it choose the physical adapter its own connections are bound to. A probe sent through the SOCKS listener in between goes out unbound and follows the routing table, where another proxy's TUN takes it, or our own routes cut it off as they appear. So a start is ready only once mihomo has logged the adapter it bound to and our adapter is up, plus a short settle for its routes; a kernel that never binds or whose adapter never appears fails the start with that reason rather than with an unexplained probe error.

**The No-Idle-Wait Rule.** Between 开始加速 and the latency appearing, every wait is on something, never on a clock. A node's request for work is held by the control service (up to `JOB_WAIT`, 8 s by default) and answered the moment a job is created, so the node's own 2 s poll no longer sits between the player and the tunnel; the service looks for the node's readiness every 0.5 s; the probe's three legs (timed requests, exit echo, UDP) run at once after the first request has opened the path; and the client asks for the state several times a second while a session is on its way in or out, two seconds apart otherwise.

**The Probe Label Rule.** Two latencies exist and are never mixed or presented as in-game latency. Line rows show a latency a player felt: this player's last session on the line, else the median of other players' sessions over the last day, and only for a line nobody has used the control plane's publish-time check, which is taken from wherever the control service runs and said 177 ms for a line whose players saw 50. The caption above the list names the source ("延迟为你上次加速时的实测值", "延迟为近期加速的实测值，标“检测值”的除外", or "延迟为线路检测值，加速后以实测为准"), a row whose figure is only the check says so after its load ("负载低 · 延迟为检测值"), and the idle summary repeats the figure after the line name with the same word ("上次 50 ms", "实测 90 ms", "检测 177 ms"). The dial, the 延迟 tile, the library banner and the chart show the running session's current tunnel probe, captioned on the chart "当前隧道检测，约每 30 秒一次": the round trip through the line to the exit's latency echo, never the leg on to the probe service that game traffic does not take (only an exit without an echo falls back to the probe service's round trip). Missing evidence is an em dash with its unit hidden; packet loss stays an em dash when the probe did not measure UDP. A stopped or new session clears the figures and the chart's history, and losing the local service replaces the pages with the disconnected state so no stale figure stays on screen.

**The Locked Session Rule.** While a session starts, runs or stops, everything that would change it locks: search, region tiles, line rows and refresh. The library stays readable: the running game's cover wears the green 加速中 badge, other covers dim to 55%, and clicking one raises a warning toast ("正在为 … 加速，停止后才能切换游戏"). Locked lists dim in place rather than turning stock white.

**The Refused Line Rule.** When the service refuses a start because of the line (full, withdrawn, node down), that line stays refused until the line list is refreshed (the refresh button beside 选择线路, or the library refresh) or three minutes pass: the row disables with a red load dot and "刚才无法使用，刷新后可重试", its latency becomes an em dash, the best remaining line is selected, and the red line under the dial names both ("… 暂时无法使用（cause），已切换到 …"), or says the region has no other usable line.

### Windows installer

The installer is the client's front door and uses its material: Inno Setup's dark style with a styled title bar and no bevels, the graphite ground (#131217) under drawn art from `installer/make-brand-assets.ps1` (a hero background with the amber glow and streaks for the start and finish pages, a quieter one for inner pages, at five DPI sizes), the mark tile beside 流光加速器 and the FLOWLIGHT ACCELERATOR tagline. The start page is one screen: a 21 pt heading, one line of what the product does, the install folder (or, in amber, the version being upgraded from), a 200 × 46 amber call to action whose label says what will happen (立即安装, 立即升级, 重新安装), and the desktop-shortcut option (an upgrade keeps whatever the player had). The server address is never shown to players, in the installer or the client; the page asking for it appears only when a build has none baked in. Installing shows the same large title and an amber bar on a #2B2833 track with its percentage; the finish page ends on 开始使用 with 立即启动 checked. When the network service did not start, the finish page says so in warm orange, names the failed step and what to do in plain words, and gives the log path; it never ends on a bare error box.

### Dialogs and feedback (web)

Editing happens in a **drawer**: a modal `<dialog>` docked to the right, 600px wide and full height (full width with square corners on phones), sliding in over the graphite backdrop so the list stays in view. Its head names the action and the record (编辑游戏与规则 over the game's name); the body is **titled sections** (an Amber Ink icon and a 15.5px Bold title, an optional note, hairlines between) of two-column fields, wide controls spanning both; a Paper hint closes the form where saving needs explaining (保存只更新草稿，点“发布规则”后客户端才会使用); the Paper footer holds 取消 and 保存. Validation runs before sending and puts each error under its field (see Inputs / Fields); the footer alert, Brick Red on its wash, is only for errors the service returns. A backdrop click never closes a form. 详情 and 查看版本与验证记录 open read-only drawers that do close on a backdrop click and show the record as label-value rows with relative times, pills and chips; verified versions offer 切换到此版本.

State changes ask first in a 440px **confirm dialog** coloured by risk: an amber info badge and a primary button to publish, switch versions, resume a line or issue enrollment credentials; a red warning badge and a danger button to drain or terminate a line, stop a session or delete a row. Deleting says what goes with the row and what blocks it; the control service refuses while anything still uses the row (endpoints on a node, lines or bindings or live sessions on an endpoint, live sessions on a line, lines limited to a game) and the dialog shows which names to clear first. Sessions, traffic and audit history keep referring to a deleted row, and the audit entry keeps its name. Once sent it says closing will not cancel the request, offers 关闭 instead of 取消, and turns the button into 重试 after an error. Results land as a banner under the page head: a green notice with a close button, or a red alert with 重试 for a failed read or action. Published profile versions, path verification, service availability and live session state remain visible as different statuses.

The reviewed captures in `.impeccable/review/` cover the redesigned administration, `admin-01-login` to `admin-21-mobile-lines`: login; the overview with tiles toned by their counts and the 上线准备 checklist; every list with entity columns and position-coloured region badges; the line list's verification states (已验证上线 beside 新版本验证中, and 验证未通过 with its reason in red); the 更多 menu with 终止线路 last (`admin-08c-line-menu`); the game, line, endpoint and user editors with region badges in tags and suggestions and errors under their fields (`admin-08b-line-editor-error`, `admin-14-user-editor-error`); the terminate confirmation, a detail drawer and an empty list; and on a phone the overview, the menu sheet and stacked row cards. `client-cover-paired-library.png` is the client library on the same test service and slot rule: 英雄联盟 blue and VALORANT red, matching `admin-03-games`. The client captures also cover the rebuilt client's library, game page, connecting and active sessions, library while running, refused start, account menu, sign-in, empty search, lost service, and minimum and maximized windows. The earlier `desktop.png`, `games-desktop.png`, `login.png`, `mobile.png` and `overview-outage.png` show the retired cyan administration. Client figures in the captures come from a test service. They establish visual and unavailable-state evidence; they do not establish successful real-game acceleration or live node availability.

The **connectivity check** drawer draws the path as a vertical rail: a card per stop (控制服务, each hop with its role pill, endpoint, address in mono, node name with 在线 in Pine Green or 不在线 with the last heartbeat in Brick Red, agent version and the test port, then 互联网 with the exit IP), and between cards a link row whose 3px rail and pill take the result's tone: Pine Green 连通, Rust 连通 · 有丢包, Brick Red 不通, grey 检测中 or 未检测. The row prints TCP and UDP as the median in the figures face with the range in brackets and any loss, or the reason in Brick Red. A banner above says 全部 N 段连通, 全部连通，N 段有丢包, or N 段不通 with the failing segments named; while it runs it is grey with a spinner and says nodes take up work every 2 seconds. 重新检查 sits beside the time of the check and is disabled while one runs.

## Do's and Don'ts

### Do:
- **Do** preserve the light administration and dark native-client surface roles, and keep the administration framed in the client's graphite.
- **Do** keep status labels, provenance, empty states, and errors visible beside the affected resource or action; in a form, under the field at fault.
- **Do** let narrow web screens reflow rather than scroll sideways (a top bar and menu sheet at 860px, stacked row cards labelled by column at 720px), and keep a fixed hero over a scrolling game page at the client's minimum size.
- **Do** use local Chinese-capable fonts: Microsoft YaHei UI for words and Bahnschrift for figures on both surfaces, and Cascadia Mono for ids and hosts on the web.
- **Do** color every client latency by one scale (60 ms or less green, to 120 ms orange, above red) and light signal bars at 60, 100 and 160 ms.
- **Do** draw new client icons as 24 px outline geometry at a 1.8 stroke with round caps, and regenerate brand rasters from the three-streak mark in `installer/make-brand-assets.ps1`.
- **Do** give each new game a generated cover from the hue at its cover_index, the same in both apps, and colour region badges by their position in the list.
- **Do** let operators replace a cover with a picture they are licensed to use; never ship publisher artwork in the repository or installer.
- **Do** lead every admin list with an entity column (picture, name, sub line), put the row's next step first as its emphasised action, and keep destructive actions last, separated and red.

### Don't:
- **Don't** replace unknown counts or missing probe evidence with zero or invented healthy measurements.
- **Don't** conflate a published rule, verified path, historical probe, and current connection.
- **Don't** claim tested game compatibility or bundle proprietary game artwork without established evidence or assets.
- **Don't** use amber to report quality or state on either surface, or a state tone as decoration; and don't bring back the retired cyan.
- **Don't** outline resting surfaces: the client separates by tone and the administration by the Rest shadow; the Float shadow is for client layers above the page.
- **Don't** pick a game's cover hue from list order or a hash when the service supplies cover_index, or renumber the slots of existing games; a game must not change colour between the two apps or when another game is added.
- **Don't** edit in a centred modal or ask with the browser's confirm(); edits go in the side drawer, state changes through the risk-coloured confirm dialog.
- **Don't** brand with a letter monogram; the brand is the three-streak amber mark, and first characters appear only inside generated covers.
- **Don't** leave a focus ring after a mouse click, or hide it from keyboard focus.
