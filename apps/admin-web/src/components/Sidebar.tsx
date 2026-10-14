import { ChevronRight, LogOut } from "lucide-react";
import type { Row } from "../api";
import { stateNames } from "../format";
import { sections } from "../sections";

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
  return (
    <aside>
      <a
        className="brand"
        href="#"
        onClick={(e) => {
          e.preventDefault();
          onSelect("overview");
        }}
      >
        <span className="brand-symbol">F</span>
        <span>
          FlowLight<strong>Accelerator</strong>
        </span>
      </a>
      <div className="workspace">
        流光加速器 <span>管理后台</span>
      </div>
      <nav aria-label="管理导航">
        {sections.map(([id, name, Icon]) => (
          <button
            key={id}
            className={section === id ? "selected" : ""}
            aria-current={section === id ? "page" : undefined}
            onClick={() => onSelect(id)}
          >
            <Icon size={18} />
            {name}
            {section === id && <ChevronRight size={14} />}
          </button>
        ))}
      </nav>
      <div className="account">
        <span>
          {user.username}
          <small>{stateNames[user.role]}</small>
        </span>
        <button title="退出登录" aria-label="退出登录" onClick={onLogout}>
          <LogOut size={17} />
        </button>
      </div>
    </aside>
  );
}
