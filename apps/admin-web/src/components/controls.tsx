import { useId, useState, type ReactNode } from "react";
import { Plus, X } from "lucide-react";

/**
 * A labelled field: label above, control, optional note below. A single
 * input sits inside a <label>; a composite control (chips, choice cards)
 * is a labelled group instead, since a label may hold only one control.
 * `name` ties the field to validation: its `error` replaces the note and
 * marks the control invalid.
 */
export function Field({
  label,
  note,
  wide,
  group,
  name,
  error,
  children,
}: {
  label: string;
  note?: ReactNode;
  wide?: boolean;
  group?: boolean;
  name?: string;
  error?: string;
  children: ReactNode;
}) {
  const className = "field" + (wide ? " wide" : "") + (error ? " invalid" : "");
  const id = useId();
  const below = error ? (
    <small className="field-error" role="alert">
      {error}
    </small>
  ) : note ? (
    <small>{note}</small>
  ) : null;
  if (group)
    return (
      <div className={className} role="group" aria-labelledby={id} aria-invalid={!!error || undefined} data-field={name}>
        <span id={id}>{label}</span>
        {children}
        {below}
      </div>
    );
  return (
    <label className={className} data-field={name}>
      <span>{label}</span>
      {children}
      {below}
    </label>
  );
}

/** An on/off setting with its explanation beside the switch. */
export function SwitchRow({
  label,
  note,
  checked,
  onChange,
}: {
  label: string;
  note?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="switch-row field wide">
      <div>
        <b>{label}</b>
        {note && <small>{note}</small>}
      </div>
      <button
        type="button"
        role="switch"
        className="switch"
        aria-checked={checked}
        aria-label={label}
        onClick={() => onChange(!checked)}
      />
    </div>
  );
}

/** One choice out of a few short options. */
export function Segmented({
  options,
  value,
  onChange,
  label,
}: {
  options: [string, string][];
  value: string;
  onChange: (v: string) => void;
  label: string;
}) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {options.map(([v, n]) => (
        <button
          type="button"
          key={v}
          aria-pressed={value === v}
          onClick={() => onChange(v)}
        >
          {n}
        </button>
      ))}
    </div>
  );
}

/** Independent on/off chips, e.g. TCP / UDP / IPv6. */
export function Toggles({
  options,
  isOn,
  onToggle,
  label,
  locked,
}: {
  options: [string, string][];
  isOn: (key: string) => boolean;
  onToggle: (key: string) => void;
  label: string;
  /** Why an option cannot be switched on right now; it stays visible. */
  locked?: (key: string) => string | undefined;
}) {
  return (
    <div className="toggles" role="group" aria-label={label}>
      {options.map(([k, n]) => {
        // An option that is on can always be switched off.
        const reason = isOn(k) ? undefined : locked?.(k);
        return (
          <button
            type="button"
            key={k}
            aria-pressed={isOn(k)}
            disabled={!!reason}
            title={reason}
            onClick={() => onToggle(k)}
          >
            {n}
          </button>
        );
      })}
    </div>
  );
}

/** One choice out of options that need a line of explanation each. */
export function RadioCards({
  options,
  value,
  onChange,
  label,
}: {
  options: [string, string, string][];
  value: string;
  onChange: (v: string) => void;
  label: string;
}) {
  return (
    <div className="radio-cards" role="radiogroup" aria-label={label}>
      {options.map(([v, title, note]) => (
        <button
          type="button"
          role="radio"
          key={v}
          className="radio-card"
          aria-checked={value === v}
          onClick={() => onChange(v)}
        >
          <b>{title}</b>
          <small>{note}</small>
        </button>
      ))}
    </div>
  );
}

/**
 * A list of short values as chips. Enter, a comma or leaving the box adds
 * what was typed; pasting several lines adds each. `suggestions` offers
 * one-click additions; `display` gives a value its readable name.
 */
export function TagInput({
  value,
  onChange,
  placeholder,
  suggestions = [],
  display,
  label,
  normalize = (s) => s,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  placeholder?: string;
  suggestions?: string[];
  /** A value's readable form; slot is its position, or -1 for a suggestion. */
  display?: (v: string, slot: number) => ReactNode;
  label: string;
  normalize?: (s: string) => string;
}) {
  const [draft, setDraft] = useState("");
  const list = value || [];
  function add(raw: string) {
    const parts = raw
      .split(/[\n,，]/)
      .map((s) => normalize(s.trim()))
      .filter(Boolean)
      .filter((s, i, a) => a.indexOf(s) === i && !list.includes(s));
    if (parts.length) onChange([...list, ...parts]);
    setDraft("");
  }
  const remaining = suggestions.filter((s) => !list.includes(s));
  return (
    <div className="field-tags">
      <div className="tags">
        {list.map((v, i) => (
          <span className="tag" key={v}>
            <span>{display ? display(v, i) : v}</span>
            <button
              type="button"
              aria-label={"移除 " + v}
              onClick={() => onChange(list.filter((x) => x !== v))}
            >
              <X size={12} />
            </button>
          </span>
        ))}
        <input
          aria-label={label}
          value={draft}
          placeholder={list.length ? "" : placeholder}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === ",") {
              e.preventDefault();
              add(draft);
            } else if (e.key === "Backspace" && !draft && list.length) {
              onChange(list.slice(0, -1));
            }
          }}
          onPaste={(e) => {
            const pasted = e.clipboardData.getData("text");
            if (/[\n,，]/.test(pasted)) {
              e.preventDefault();
              add(draft + pasted);
            }
          }}
          onBlur={() => draft.trim() && add(draft)}
        />
      </div>
      {remaining.length > 0 && (
        <div className="suggest" style={{ marginTop: 8 }}>
          {remaining.map((s) => (
            <button type="button" key={s} onClick={() => onChange([...list, s])}>
              <Plus size={12} />
              {display ? display(s, -1) : s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/** An option of a picker: its id, its name, and an optional picture. */
export type PickOption = { value: string; label: string; lead?: ReactNode };

/**
 * Several rows picked by name; stores their ids. Typing filters the
 * choices, which are offered as one-click chips like the region picker.
 */
export function PickList({
  value,
  onChange,
  options,
  empty,
  label,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  options: PickOption[];
  empty: string;
  label: string;
}) {
  const [query, setQuery] = useState("");
  const list = value || [];
  const find = (id: string) => options.find((o) => o.value === id);
  const q = query.trim().toLowerCase();
  const remaining = options.filter(
    (o) => !list.includes(o.value) && (!q || o.label.toLowerCase().includes(q)),
  );
  const shown = remaining.slice(0, 12);
  return (
    <div className="field-tags">
      <div className="tags">
        {list.map((v) => {
          const o = find(v);
          return (
            <span className="tag" key={v}>
              {o?.lead}
              <span title={v}>{o?.label || v}</span>
              <button type="button" aria-label={"移除 " + (o?.label || v)} onClick={() => onChange(list.filter((x) => x !== v))}>
                <X size={12} />
              </button>
            </span>
          );
        })}
        <input
          aria-label={label}
          value={query}
          placeholder={list.length ? "搜索并添加…" : empty + "；输入名称搜索"}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              if (shown[0]) {
                onChange([...list, shown[0].value]);
                setQuery("");
              }
            } else if (e.key === "Backspace" && !query && list.length) {
              onChange(list.slice(0, -1));
            }
          }}
        />
      </div>
      {shown.length > 0 && (
        <div className="suggest" style={{ marginTop: 8 }}>
          {shown.map((o) => (
            <button
              type="button"
              key={o.value}
              onClick={() => {
                onChange([...list, o.value]);
                setQuery("");
              }}
            >
              <Plus size={12} />
              {o.lead}
              {o.label}
            </button>
          ))}
          {remaining.length > shown.length && (
            <span className="muted" style={{ fontSize: 12, alignSelf: "center" }}>
              还有 {remaining.length - shown.length} 个，输入名称查找
            </span>
          )}
        </div>
      )}
      {q && remaining.length === 0 && (
        <small className="muted" style={{ display: "block", marginTop: 6, fontSize: 12 }}>
          没有匹配“{query.trim()}”的选项
        </small>
      )}
    </div>
  );
}
