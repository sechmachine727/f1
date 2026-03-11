import { useMemo } from "react";
import { Map } from "lucide-react";
import { useTrackMap } from "@/hooks/useTrackMap";

export function TrackMap() {
  const state = useTrackMap();

  const hasOutline = state && state.trackOutline.length >= 10;
  const hasCars = state && state.cars.some(c => c.active);

  if (!hasOutline) {
    return (
      <div className="bg-card border border-border/50 rounded-md overflow-hidden h-full flex flex-col">
        <Header trackName="" carCount={0} />
        <div className="flex-1 flex items-center justify-center">
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-display">
            Waiting for track data…
          </span>
        </div>
      </div>
    );
  }

  return <TrackMapSVG state={state!} hasCars={hasCars ?? false} />;
}

function TrackMapSVG({ state, hasCars }: { state: NonNullable<ReturnType<typeof useTrackMap>>; hasCars: boolean }) {
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
    const padX = Math.max((maxX - minX) * 0.1, 20);
    const padZ = Math.max((maxZ - minZ) * 0.1, 20);
    return {
      x: minX - padX,
      z: minZ - padZ,
      w: maxX - minX + 2 * padX,
      h: maxZ - minZ + 2 * padZ,
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
  const trackStroke = scale * 0.005;
  const otherR = scale * 0.007;
  const playerR = scale * 0.012;
  const labelSize = scale * 0.018;

  return (
    <div className="bg-card border border-border/50 rounded-md overflow-hidden h-full flex flex-col">
      <Header trackName={trackName} carCount={hasCars ? carCount : 0} />
      <div className="flex-1 p-2 min-h-0">
        <svg
          viewBox={`${bounds.x} ${bounds.z} ${bounds.w} ${bounds.h}`}
          className="w-full h-full"
          preserveAspectRatio="xMidYMid meet"
        >
          <defs>
            <filter id="playerGlow" x="-100%" y="-100%" width="300%" height="300%">
              <feGaussianBlur stdDeviation={scale * 0.005} result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
            <filter id="trackGlow" x="-10%" y="-10%" width="120%" height="120%">
              <feGaussianBlur stdDeviation={scale * 0.002} result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          {/* Track outline */}
          {outlineComplete ? (
            <polygon
              points={outlinePoints}
              fill="none"
              className="stroke-primary"
              strokeWidth={trackStroke}
              strokeOpacity={0.35}
              strokeLinejoin="round"
              filter="url(#trackGlow)"
            />
          ) : (
            <polyline
              points={outlinePoints}
              fill="none"
              className="stroke-primary"
              strokeWidth={trackStroke}
              strokeOpacity={0.25}
              strokeLinejoin="round"
              strokeLinecap="round"
              filter="url(#trackGlow)"
            />
          )}

          {/* Other cars */}
          {activeCars.map((car, i) => (
            <g key={i}>
              <circle
                cx={car.x}
                cy={car.z}
                r={otherR}
                className="fill-muted-foreground"
                fillOpacity={0.55}
              />
              {car.position > 0 && (
                <text
                  x={car.x}
                  y={car.z - otherR * 2.2}
                  textAnchor="middle"
                  className="fill-muted-foreground"
                  fontSize={labelSize}
                  fontFamily="monospace"
                  fillOpacity={0.45}
                >
                  {car.position}
                </text>
              )}
            </g>
          ))}

          {/* Player car — on top */}
          {playerCar && (playerCar.x !== 0 || playerCar.z !== 0) && (
            <g filter="url(#playerGlow)">
              <circle
                cx={playerCar.x}
                cy={playerCar.z}
                r={playerR}
                className="fill-accent"
              />
            </g>
          )}
        </svg>
      </div>
    </div>
  );
}

function Header({ trackName, carCount }: { trackName: string; carCount: number }) {
  return (
    <div className="flex items-center gap-2 px-3 py-2 border-b border-border/50 bg-secondary/30">
      <Map className="h-4 w-4 text-primary" />
      <span className="font-display text-sm font-bold tracking-widest uppercase text-foreground">
        Track Map
      </span>
      {carCount > 0 && (
        <span className="ml-auto font-display text-[10px] text-muted-foreground">
          {carCount} CARS
        </span>
      )}
    </div>
  );
}
