---
name: FlowLight Accelerator
description: Light operational administration and a dark native Windows game library.
colors:
  admin-accent: "#087e93"
  admin-background: "#f6f8fa"
  admin-surface: "#ffffff"
  admin-text: "#182433"
  admin-muted: "#677487"
  admin-line: "#e2e7ec"
  client-accent: "#31c5d7"
  client-background: "#0c121c"
  client-surface: "#151d29"
  client-text: "#e7edf5"
  client-muted: "#a2afbf"
  client-field: "#1d2838"
  client-button: "#263244"
  success-background: "#e5f4ed"
  success-text: "#216944"
  failure-background: "#fff0ed"
  failure-text: "#ac4934"
  progress-background: "#e8f2fc"
  progress-text: "#34638d"
typography:
  admin-body:
    fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif'
    fontSize: "13px"
    lineHeight: 1.8
  admin-label:
    fontFamily: '"Segoe UI", "Microsoft YaHei", sans-serif'
    fontSize: "12px"
  client-body:
    fontFamily: "Segoe UI, Microsoft YaHei"
    fontSize: "14px"
  client-label:
    fontFamily: "Segoe UI, Microsoft YaHei"
    fontSize: "12px"
rounded:
  chip: "4px"
  control: "7px"
  container: "9px"
  dialog: "12px"
spacing:
  compact: "8px"
  control-gap: "10px"
  label-gap: "12px"
  section-gap: "20px"
  section: "24px"
  desktop-content: "36px"
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
    backgroundColor: "{colors.client-button}"
    textColor: "{colors.client-text}"
    rounded: "{rounded.control}"
    padding: "11px 18px"
  button-client-primary:
    backgroundColor: "{colors.client-accent}"
    textColor: "#07181c"
    rounded: "{rounded.control}"
    padding: "11px 18px"
---

# Design System: FlowLight Accelerator

## Overview

**Creative North Star: "Operate"**

FlowLight uses light operational tables for administrators and a dark game library with a connection panel for Windows players. Cyan identifies actions and selection. This record merges the confirmed incumbent direction with the implemented React administration and native WPF client; it does not introduce a new visual concept.

Dense resource views, plain language, and visible connection evidence support the initial VALORANT and League of Legends workflow. Game names and simple vector icons provide recognition; no game artwork is bundled.

**Key Characteristics:**
- Light administration, dark native client, cyan actions.
- Persistent navigation, compact resource tables, modal editors.
- Game selection beside one connection panel and its start action.
- Unknown, draft, unavailable, historical, and current states remain explicit.

## Colors

### Primary

Admin accent is the deeper cyan used for primary buttons, inline actions, and navigation cues. Client accent is the brighter cyan used for start and login actions on dark surfaces. The two accents share a role while retaining their surface-specific contrast.

### Neutral

Admin background surrounds white surfaces; dark text, muted supporting text, and fine dividers establish hierarchy. Client background surrounds raised dark surfaces; light text and muted labels establish the same hierarchy. Dark fields and secondary buttons remain distinct from their containers.

Success, failure, and progress color pairs are state indicators in administration badges. Labels carry the state alongside color.

**The Evidence Rule.** A color or count must describe reported state: an enabled resource, a published profile, a verified path, and a live connection are separate facts.

## Typography

The shipped local stack is Segoe UI with Microsoft YaHei fallback; web adds sans-serif. There are no downloaded font files or external font requests. Preserve readable Chinese and Latin text in both interfaces.

Web tables and supporting labels use compact type (11–13px); standard prose and actions use body-sized type (13px), with prose line height (1.8). Page headings are (27px), reducing to (23px) on narrow screens; section headings are (20px). Overview numerals use tabular figures (34px), reducing to (28px) on mobile.

WPF body text is (14 DIPs), with supporting labels (11–12 DIPs), section titles (17 DIPs), game titles (22 DIPs), and the window heading (28 DIPs). Probe numerals use (32 DIPs). These are observed roles, not a mathematical type scale. The current system-font display treatment and tiny brand/version captions are recorded as incumbent details, not a branded display or small-text standard for future surfaces.

## Layout

Administration uses a fixed-width sidebar (232px), a flexible content column, header (70px), and content padding (36px), capped at (1720px). At a viewport width of (1000px) or less, the sidebar becomes (190px), content padding becomes (26px), and the four overview metrics become two columns.

At (680px) or less, navigation moves above content into a horizontally scrollable row, page actions stack below the title, content padding becomes (24px 16px), and editor fields become one column. Tables keep their readable minimum width (640px) inside a horizontally scrollable container. Login switches from two columns to the form alone. Dialogs fit the viewport with (16px) side clearance and a maximum height of (90vh).

The WPF window starts at (1120 × 750 DIPs), with a minimum of (900 × 650 DIPs). It retains its sidebar (190 DIPs) and connection panel (280 DIPs); the game library absorbs width changes. The connection panel scrolls its controls and evidence while keeping start and stop actions at the bottom. This is a native minimum-size layout, not a mobile breakpoint layout.

## Elevation & Depth

Both applications use surface contrast and thin borders; no custom drop-shadow vocabulary is defined. The browser dialog uses a dim backdrop (`rgb(16 32 46 / 35%)`), while the native login uses a dark translucent overlay. Interaction feedback is immediate color, outline, border, or opacity change; no custom animation timing is defined. Web reduced-motion preferences disable transitions.

## Shapes

Controls use modest corners, resource containers slightly broader corners, and dialogs the broadest corners, as listed in the tokens. Compact state badges use the smallest corners. Table rows are divided by fine horizontal rules. Simple SVG icons in the web app and XAML paths in the native app communicate navigation, search, arrows, and actions; the F monogram remains text branding.

## Components

### Buttons

Web primary actions use cyan with white text; secondary actions use white with a border. Hover darkens the primary or tints the secondary. Keyboard focus uses a visible outline (3px with 3px offset); disabled buttons use reduced opacity (0.5). Native buttons use dark secondary or bright cyan primary fills, lower opacity on hover (0.86), a cyan keyboard border (2 DIPs), and disabled opacity (0.4).

### Inputs / Fields

Web fields use white fill, a cool gray border, and visible focus outline (2px with 1px offset). Search includes a vector magnifier; labels stay visible above editor fields. Native fields and dropdowns use dark fills, light text, and contrasting borders. Native dropdowns have a minimum height (40 DIPs), with separate highlighted and selected item fills.

### Navigation

Desktop administration navigation stays visible beside the content; selection uses a pale cyan fill, stronger text, and a directional icon. Its narrow layout scrolls horizontally. Native navigation stays in the left rail with account and exit controls at the bottom.

### Chips

Small filled badges display operational state text. Green denotes active/succeeded/ready or enabled flags, warm red failure/degradation, and blue work in progress. The badge's label determines what the color claims; an enabled flag alone does not imply a verified connection.

### Cards / Containers

Resource tables use white bordered containers with subdued headers, row hover tint, and inline actions. Metrics share one segmented container. Empty tables retain their column context and explain loading, no records, or no matching search results. Errors appear in a separate alert with retry; unknown overview counts remain an em dash during loading and after failure.

Native game cards use a dark fill and border; selection changes both to cyan-tinted treatments. The adjacent connection panel groups region, line, probe evidence, and start/stop controls. A missing core, unpublished game rule, absent region, or unavailable authorized line keeps start disabled and is explained in nearby status copy.

**The Probe Label Rule.** Historical line measurements are labeled “线路历史探测”; current tunnel evidence is labeled “当前隧道探测”. Missing evidence displays an em dash, and service loss clears stale measurements. Neither measurement is presented as in-game latency.

### Dialogs and feedback

Web editors use native dialogs, two-column fields where space permits, and a footer with cancel and save. Validation errors stay within the form. Native login is a centered overlay with inline error text. Published profile versions, path verification, service availability, and live session state remain visible as different statuses.

The reviewed captures in `.impeccable/review/` cover desktop administration, game resources, narrow web layout, native normal/minimum sizes, login, and overview outage. These establish visual and unavailable-state evidence; they do not establish successful real-game acceleration or live node availability.

## Do's and Don'ts

### Do:
- **Do** preserve the light administration and dark native-client surface roles.
- **Do** keep status labels, provenance, empty states, and errors visible beside the affected resource or action.
- **Do** retain horizontally scrollable tables on narrow web screens and scrollable connection details at the native minimum size.
- **Do** use local Chinese-capable fonts and simple vector icons already present in the applications.

### Don't:
- **Don't** replace unknown counts or missing probe evidence with zero or invented healthy measurements.
- **Don't** conflate a published rule, verified path, historical probe, and current connection.
- **Don't** claim tested game compatibility or bundle proprietary game artwork without established evidence or assets.
