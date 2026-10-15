import { useState } from "react";
import { LogOut, Menu, X } from "lucide-react";
import type { Row } from "../api";
import { stateNames } from "../format";
import { sections, sectionTitle } from "../sections";
import { Avatar } from "../visuals";
import { BrandMark } from "./BrandMark";

export function Sidebar({
  section,
  user,
  onSelect,
  onLogout,
}: {
  section: string;
  user: Row;
  onSelect: (id: string) => void;
  onLogout: () => void;
}) {
  // On narrow screens the navigation is a sheet opened from the top bar.
  const [open, setOpen] = useState(false);
  const groups: [string, (typeof sections)[number][]][] = [];
  for (const entry of sections) {
    const group = entry[3];
    const last = groups[groups.length - 1];
    if (last && last[0] === group) last[1].push(entry);
    else groups.push([group, [entry]]);
  }
  const select = (id: string) => {
    setOpen(false);
    onSelect(id);
  };
  return (
    <aside className={"side" + (open ? " open" : "")}>
      <div className="side-top">
        <a
          className="brand"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            select("overview");
          }}
        >
          <BrandMark />
          <span className="brand-name">
            流光加速器
            <small>FLOWLIGHT ADMIN</small>
          </span>
        </a>
        <span className="side-current">{sectionTitle(section)}</span>
        <button
          className="icon-btn side-toggle"
          aria-label={open ? "关闭导航" : "打开导航"}
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
        >
          {open ? <X size={20} /> : <Menu size={20} />}
        </button>
      </div>
      <div className="side-body">
        <nav aria-label="管理导航">
          {groups.map(([group, entries]) => (
            <div className="side-group" key={group || "home"}>
              {group && <span>{group}</span>}
              {entries.map(([id, name, Icon]) => (
                <button
                  key={id}
                  className={"nav-item" + (section === id ? " active" : "")}
                  aria-current={section === id ? "page" : undefined}
                  onClick={() => select(id)}
                >
                  <Icon size={18} />
                  {name}
                </button>
              ))}
            </div>
          ))}
        </nav>
        <div className="side-account">
          <Avatar name={user.username} />
          <div>
            <b>{user.username}</b>
            <small>{stateNames[user.role] || user.role}</small>
          </div>
          <button className="icon-btn" title="退出登录" aria-label="退出登录" onClick={onLogout}>
            <LogOut size={17} />
          </button>
        </div>
      </div>
    </aside>
  );
}
