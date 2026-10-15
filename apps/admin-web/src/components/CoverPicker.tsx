import { useRef, useState } from "react";
import { RotateCcw, Trash2, Upload } from "lucide-react";
import { Cover } from "../visuals";

/** A cover change made in the editor, applied when the game is saved. */
export type CoverChange = { file: File; preview: string } | { clear: true };

const accepted = ["image/jpeg", "image/png", "image/webp"];
const maxBytes = 10 * 1024 * 1024;

function readDataUrl(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

function pictureSize(src: string) {
  return new Promise<{ w: number; h: number } | null>((resolve) => {
    const img = new Image();
    img.onload = () => resolve({ w: img.naturalWidth, h: img.naturalHeight });
    img.onerror = () => resolve(null);
    img.src = src;
  });
}

/**
 * The game's cover picture, previewed as the client shows it: a library
 * card and the wide banner on the game page. The control service crops
 * uploads to 16:9 around the centre, as the card preview does.
 */
export function CoverPicker({
  seed,
  name,
  index,
  current,
  change,
  onChange,
}: {
  seed: string;
  name: string;
  index?: unknown;
  /** The stored picture's name, if any. */
  current?: string;
  change: CoverChange | null;
  onChange: (c: CoverChange | null) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [problem, setProblem] = useState("");
  const [warning, setWarning] = useState("");
  const shown = change ? ("file" in change ? change.preview : undefined) : current || undefined;
  const title = name.trim() || "游戏名称";

  async function choose(file: File | undefined) {
    setProblem("");
    setWarning("");
    if (!file) return;
    if (!accepted.includes(file.type)) return setProblem("请选择 JPG、PNG 或 WebP 格式的图片");
    if (file.size > maxBytes) return setProblem("图片超过 10 MB，请压缩后再上传");
    const preview = await readDataUrl(file).catch(() => "");
    const size = preview ? await pictureSize(preview) : null;
    if (!size) return setProblem("无法读取这张图片，请换一张");
    if (size.w < 640 || size.h < 360)
      return setProblem(`图片太小（${size.w}×${size.h}）：至少 640×360，建议 1920×1080`);
    const ratio = size.w / size.h;
    if (ratio < 1.5 || ratio > 2.1)
      setWarning("这张图不是 16:9，" + (ratio < 1.5 ? "上下" : "左右") + "会被裁掉一部分，请看预览确认。");
    onChange({ file, preview });
  }

  return (
    <div className="cover-picker">
      <div className="cover-previews">
        <figure className="preview-card">
          <div className="preview-art">
            <Cover seed={seed} name={title} index={index} image={shown} />
          </div>
          <b>{title}</b>
          <figcaption>游戏库卡片</figcaption>
        </figure>
        <figure className="preview-banner">
          <div className="preview-art">
            <Cover seed={seed} name={title} index={index} image={shown} banner />
            <b>{title}</b>
          </div>
          <figcaption>游戏页横幅</figcaption>
        </figure>
      </div>
      <div className="cover-actions">
        <input
          ref={input}
          type="file"
          accept={accepted.join(",")}
          hidden
          onChange={(e) => {
            void choose(e.target.files?.[0]);
            e.target.value = "";
          }}
        />
        <button type="button" className="secondary sm" onClick={() => input.current?.click()}>
          <Upload size={14} />
          {shown ? "更换图片" : "上传图片"}
        </button>
        {shown && current && (
          <button type="button" className="ghost sm" onClick={() => onChange({ clear: true })}>
            <Trash2 size={14} />
            使用默认封面
          </button>
        )}
        {change && (
          <button
            type="button"
            className="ghost sm"
            onClick={() => {
              setProblem("");
              setWarning("");
              onChange(null);
            }}
          >
            <RotateCcw size={14} />
            撤销更改
          </button>
        )}
      </div>
      {problem ? (
        <small className="field-error" role="alert">
          {problem}
        </small>
      ) : warning ? (
        <small className="cover-warning">{warning}</small>
      ) : null}
      <small className="cover-note">
        建议 1920×1080 的横版图，自动居中裁成 16:9；支持 JPG、PNG、WebP，不超过 10 MB。请使用有授权的图片。保存后客户端刷新游戏库即可看到，不需要发布规则。
      </small>
    </div>
  );
}
