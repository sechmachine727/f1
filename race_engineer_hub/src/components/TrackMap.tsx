import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Compass, Map } from "lucide-react";
import { useTrackMap } from "@/hooks/useTrackMap";

const ROTATION_STORAGE_KEY = "trackMap:rotations";

function loadSavedRotation(trackName: string): number {
  try {
    const stored = localStorage.getItem(ROTATION_STORAGE_KEY);
    if (stored) {
      const map = JSON.parse(stored) as Record<string, number>;
      return map[trackName] ?? 0;
    }
  } catch { /* ignore */ }
  return 0;
}

function saveRotation(trackName: string, deg: number) {
  try {
    const stored = localStorage.getItem(ROTATION_STORAGE_KEY);
    const map: Record<string, number> = stored ? JSON.parse(stored) : {};
    map[trackName] = deg;
    localStorage.setItem(ROTATION_STORAGE_KEY, JSON.stringify(map));
  } catch { /* ignore */ }
}

/** Build an SVG polygon points string for a triangular arrow at (cx, cz) pointing in `heading`. */
function playerArrow(cx: number, cz: number, heading: number, size: number): string {
  const cos = Math.cos(heading);
  const sin = Math.sin(heading);
  const tipX = cx + cos * size;
  const tipZ = cz + sin * size;
  const rearLX = cx - cos * size * 0.5 + sin * size * 0.55;
  const rearLZ = cz - sin * size * 0.5 - cos * size * 0.55;
  const rearRX = cx - cos * size * 0.5 - sin * size * 0.55;
  const rearRZ = cz - sin * size * 0.5 + cos * size * 0.55;
  return `${tipX},${tipZ} ${rearLX},${rearLZ} ${rearRX},${rearRZ}`;
}

export function TrackMap() {
  const state = useTrackMap();
  const [rotation, setRotation] = useState(0);
  const trackNameRef = useRef("");

  const trackName = state?.trackName ?? "";

  // Load saved rotation when track changes
  useEffect(() => {
    if (trackName && trackName !== trackNameRef.current) {
      trackNameRef.current = trackName;
      setRotation(loadSavedRotation(trackName));
    }
  }, [trackName]);

  const handleRotationChange = useCallback((deg: number) => {
    setRotation(deg);
    if (trackName) saveRotation(trackName, deg);
  }, [trackName]);

  const hasOutline = state && state.trackOutline.length >= 10;
  const hasCars = state && state.cars.some(c => c.active);

  if (!hasOutline) {
    return (
      <div className="bg-card border border-border/50 rounded-md overflow-hidden h-full flex flex-col">
        <Header trackName="" carCount={0} rotation={0} onRotationChange={() => {}} />
        <div className="flex-1 flex items-center justify-center">
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-display">
            Waiting for track data…
          </span>
        </div>
      </div>
    );
  }

  return <TrackMapSVG state={state!} hasCars={hasCars ?? false} rotation={rotation} onRotationChange={handleRotationChange} />;
}

function TrackMapSVG({
  state,
  hasCars,
  rotation,
  onRotationChange,
}: {
  state: NonNullable<ReturnType<typeof useTrackMap>>;
  hasCars: boolean;
  rotation: number;
  onRotationChange: (deg: number) => void;
}) {
  const { trackOutline, cars, playerIndex, outlineComplete, trackName } = state;

  const activeCars = useMemo(
    () => cars.filter((c, i) => c.active && i !== playerIndex),
    [cars, playerIndex],
  );
  const playerCar = cars[playerIndex];
  const carCount = activeCars.length + (playerCar && (playerCar.x !== 0 || playerCar.z !== 0) ? 1 : 0);

  // Compute bounding box from the track outline
  const bounds = useMemo(() => {
    let minX = Infinity, maxX = -Infinity;
    let minZ = Infinity, maxZ = -Infinity;
    for (const p of trackOutline) {
      if (p.x < minX) minX = p.x;
      if (p.x > maxX) maxX = p.x;
      if (p.z < minZ) minZ = p.z;
      if (p.z > maxZ) maxZ = p.z;
    }
    const padX = Math.max((maxX - minX) * 0.04, 10);
    const padZ = Math.max((maxZ - minZ) * 0.04, 10);
    return {
      x: minX - padX,
      z: minZ - padZ,
      w: maxX - minX + 2 * padX,
      h: maxZ - minZ + 2 * padZ,
      cx: (minX + maxX) / 2,
      cz: (minZ + maxZ) / 2,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trackOutline.length, outlineComplete]);

  // Build SVG points string
  const outlinePoints = useMemo(() => {
    return trackOutline.map(p => `${p.x},${p.z}`).join(" ");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trackOutline.length, outlineComplete]);

  // Scale-relative sizes
  const scale = Math.max(bounds.w, bounds.h);
  const trackWidth = scale * 0.03;
  const centerLineWidth = scale * 0.012;
  const otherR = scale * 0.012;
  const playerSize = scale * 0.018;
  const labelSize = scale * 0.022;

  const TrackOutline = outlineComplete ? "polygon" : "polyline";

  return (
    <div className="bg-card border border-border/50 rounded-md overflow-hidden h-full flex flex-col">
      <Header trackName={trackName} carCount={hasCars ? carCount : 0} rotation={rotation} onRotationChange={onRotationChange} />
      <div className="flex-1 p-1 min-h-0 overflow-hidden">
        <svg
          viewBox={`${bounds.x} ${bounds.z} ${bounds.w} ${bounds.h}`}
          className="w-full h-full"
          preserveAspectRatio="xMidYMid meet"
        >
          <defs>
            <filter id="playerGlow" x="-200%" y="-200%" width="500%" height="500%">
              <feGaussianBlur stdDeviation={scale * 0.006} result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
            <filter id="trackGlow" x="-10%" y="-10%" width="120%" height="120%">
              <feGaussianBlur stdDeviation={scale * 0.003} result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <g transform={`rotate(${rotation} ${bounds.cx} ${bounds.cz})`}>
            {/* Track — thick dark green */}
            <TrackOutline
              points={outlinePoints}
              fill="none"
              stroke="#166534"
              strokeWidth={trackWidth}
              strokeLinejoin="round"
              strokeLinecap="round"
              filter="url(#trackGlow)"
            />

            {/* Center line — thin light white */}
            <TrackOutline
              points={outlinePoints}
              fill="none"
              stroke="#ffffff"
              strokeWidth={centerLineWidth}
              strokeOpacity={1}
              strokeLinejoin="round"
              strokeLinecap="round"
            />

            {/* Other cars */}
            {activeCars.map((car, i) => (
              <g key={i}>
                <circle
                  cx={car.x}
                  cy={car.z}
                  r={otherR}
                  fill="none"
                  stroke="#4b5563"
                  strokeWidth={otherR * 0.8}
                />
                {car.position > 0 && (
                  <text
                    x={car.x}
                    y={car.z - otherR * 2.2}
                    textAnchor="middle"
                    fill="#d1d5db"
                    fontSize={labelSize}
                    fontFamily="monospace"
                    fontWeight="bold"
                    transform={`rotate(${-rotation} ${car.x} ${car.z})`}
                  >
                    {car.position}
                  </text>
                )}
              </g>
            ))}

            {/* Player car — glowing red arrow pointing in driving direction */}
            {playerCar && (playerCar.x !== 0 || playerCar.z !== 0) && (
              <g filter="url(#playerGlow)">
                <polygon
                  points={playerArrow(playerCar.x, playerCar.z, playerCar.heading, playerSize)}
                  fill="#ef4444"
                />
              </g>
            )}
          </g>
        </svg>
      </div>
    </div>
  );
}

/** Draggable compass rotation control. */
function RotationDial({ rotation, onChange }: { rotation: number; onChange: (deg: number) => void }) {
  const dialRef = useRef<HTMLDivElement>(null);
  const draggingRef = useRef(false);
  const startAngleRef = useRef(0);
  const startRotationRef = useRef(0);

  const getAngleFromEvent = useCallback((e: { clientX: number; clientY: number }) => {
    const el = dialRef.current;
    if (!el) return 0;
    const rect = el.getBoundingClientRect();
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2;
    return Math.atan2(e.clientY - cy, e.clientX - cx) * (180 / Math.PI);
  }, []);

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    e.preventDefault();
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    draggingRef.current = true;
    startAngleRef.current = getAngleFromEvent(e);
    startRotationRef.current = rotation;
  }, [getAngleFromEvent, rotation]);

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!draggingRef.current) return;
    const currentAngle = getAngleFromEvent(e);
    const delta = currentAngle - startAngleRef.current;
    const newRotation = Math.round(startRotationRef.current + delta) % 360;
    onChange(newRotation);
  }, [getAngleFromEvent, onChange]);

  const onPointerUp = useCallback(() => {
    draggingRef.current = false;
  }, []);

  return (
    <div
      ref={dialRef}
      className="relative w-6 h-6 cursor-grab active:cursor-grabbing select-none"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      title={`Rotate track (${rotation}°)`}
    >
      <Compass
        className="w-6 h-6 text-muted-foreground hover:text-primary transition-colors"
        style={{ transform: `rotate(${rotation}deg)` }}
      />
    </div>
  );
}

function Header({
  trackName,
  carCount,
  rotation,
  onRotationChange,
}: {
  trackName: string;
  carCount: number;
  rotation: number;
  onRotationChange: (deg: number) => void;
}) {
  return (
    <div className="flex items-center gap-2 px-3 py-2 border-b border-border/50 bg-secondary/30">
      <Map className="h-4 w-4 text-primary" />
      <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">
        Track Map
      </span>
      <div className="ml-auto flex items-center gap-2">
        <RotationDial rotation={rotation} onChange={onRotationChange} />
        {carCount > 0 && (
          <span className="font-display text-[10px] text-muted-foreground">
            {carCount} CARS
          </span>
        )}
      </div>
    </div>
  );
}
