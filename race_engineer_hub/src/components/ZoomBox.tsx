import { useEffect, useRef, useState, type ReactNode } from "react";

type Props = {
  count: number;
  labels?: (string | number)[];
  fill?: boolean;
  children: (from: number, to: number) => ReactNode;
};

type Win = { from: number; to: number };

function clampWin(w: Win, count: number): Win {
  const max = Math.max(0, count - 1);
  const width = Math.min(max, Math.max(1, w.to - w.from));
  const from = Math.min(Math.max(0, w.from), max - width);
  return { from, to: Math.min(max, from + width) };
}

/**
 * Index window for the lap-axis charts: 100 percent shows every point, wheel or
 * buttons zoom, drag or arrows pan, and reset returns to the full range.
 */
export function ZoomBox({ count, labels, fill, children }: Props) {
  const [win, setWin] = useState<Win | null>(null);
  const plot = useRef<HTMLDivElement>(null);
  const drag = useRef<{ x: number; from: number; to: number } | null>(null);
  const full = { from: 0, to: Math.max(0, count - 1) };
  const w = win && count > 1 ? clampWin(win, count) : full;
  const width = w.to - w.from;
  const percent = count <= 1 ? 100 : Math.round((Math.max(0, count - 1) / Math.max(1, width)) * 100);

  const zoom = (factor: number, ratio = 0.5) => {
    if (count <= 1) return;
    const next = Math.max(1, Math.min(count - 1, Math.round(width / factor)));
    const anchor = w.from + ratio * width;
    const from = Math.round(anchor - ratio * next);
    setWin(clampWin({ from, to: from + next }, count));
  };
  const move = (by: number) => setWin(clampWin({ from: w.from + by, to: w.to + by }, count));
  const reset = () => setWin(null);

  useEffect(() => {
    const el = plot.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (count <= 1) return;
      e.preventDefault();
      const rect = el.getBoundingClientRect();
      const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / Math.max(1, rect.width)));
      const factor = e.deltaY < 0 ? 1.25 : 1 / 1.25;
      const next = Math.max(1, Math.min(count - 1, Math.round(width / factor)));
      const anchor = w.from + ratio * width;
      const from = Math.round(anchor - ratio * next);
      setWin(clampWin({ from, to: from + next }, count));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [count, width, w.from]);

  const span = labels ? labels[w.from] + "–" + labels[w.to] : w.from + 1 + "–" + (w.to + 1) + " of " + count;
  return (
    <div role="group" aria-label="Zoomable chart" tabIndex={0} className={"outline-offset-2" + (fill ? " flex h-full flex-col" : "")}
      onKeyDown={(e) => {
        if (e.key === "+" || e.key === "=") { zoom(1.5); e.preventDefault(); }
        else if (e.key === "-" || e.key === "_") { zoom(1 / 1.5); e.preventDefault(); }
        else if (e.key === "ArrowLeft") { move(-Math.max(1, Math.round(width * 0.1))); e.preventDefault(); }
        else if (e.key === "ArrowRight") { move(Math.max(1, Math.round(width * 0.1))); e.preventDefault(); }
        else if (e.key === "0") { reset(); e.preventDefault(); }
      }}>
      <div className="flex items-center gap-2 px-1 pb-1 text-[10px] text-neutral-400">
        <span className="tabular-nums">{span}</span>
        <span>{percent}%</span>
        <div className="ml-auto flex items-center gap-1">
          <button type="button" aria-label="Zoom out" className="px-1.5 text-sm text-neutral-300 hover:bg-[#1a2027]" onClick={() => zoom(1 / 1.5)}>−</button>
          <button type="button" aria-label="Zoom in" className="px-1.5 text-sm text-neutral-300 hover:bg-[#1a2027]" onClick={() => zoom(1.5)}>+</button>
          <button type="button" className="px-1.5 text-neutral-300 hover:bg-[#1a2027]" onClick={reset}>Reset</button>
        </div>
      </div>
      <div ref={plot} className={"cursor-grab" + (fill ? " min-h-0 flex-1" : "")}
        onPointerDown={(e) => { drag.current = { x: e.clientX, from: w.from, to: w.to }; }}
        onPointerMove={(e) => {
          if (!drag.current || !plot.current) return;
          const px = Math.max(1, plot.current.getBoundingClientRect().width);
          const by = Math.round(((drag.current.x - e.clientX) / px) * (drag.current.to - drag.current.from));
          setWin(clampWin({ from: drag.current.from + by, to: drag.current.to + by }, count));
        }}
        onPointerUp={() => { drag.current = null; }}
        onPointerLeave={() => { drag.current = null; }}>
        {children(w.from, w.to)}
      </div>
    </div>
  );
}

