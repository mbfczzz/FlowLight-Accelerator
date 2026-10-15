/**
 * The FlowLight mark: three amber light streaks on a graphite tile, the same
 * drawing as the Windows client, its icon and the installer.
 */
export function BrandMark({ size = 36 }: { size?: number }) {
  return (
    <svg
      className="brand-mark"
      width={size}
      height={size}
      viewBox="0 0 36 36"
      aria-hidden="true"
    >
      <defs>
        <linearGradient id="flowlight-tile" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#2a2733" />
          <stop offset="1" stopColor="#121116" />
        </linearGradient>
        <linearGradient id="flowlight-streak" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#f7cb72" />
          <stop offset="1" stopColor="#e6ae45" />
        </linearGradient>
      </defs>
      <rect width="36" height="36" rx="8" fill="url(#flowlight-tile)" />
      <g fill="url(#flowlight-streak)">
        <path d="M8.2 10.4h13.4l-1.7 3.1H10.2z" />
        <path d="M12.2 16.4h15.6l-1.7 3.1H13.9z" />
        <path d="M8.2 22.4h10.6l-1.7 3.1H10.2z" />
      </g>
    </svg>
  );
}
