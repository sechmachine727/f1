export interface CornerInfo {
  number: number;
  letter: string;
  angle: number;
  x: number;
  y: number;
}

export interface MarshalSectorInfo {
  number: number;
  x: number;
  y: number;
}

export interface MarshalZone {
  zoneStart: number;
  zoneFlag: number;
}

export interface SectorBoundaries {
  sector2Start: number;
  sector3Start: number;
}

export interface PlayerDrs {
  drsActive: boolean;
  drsAllowed: boolean;
  drsActivationDistance: number;
}

export interface CircuitInfo {
  name: string;
  rotation: number;
  outline: [number, number][];
  corners: CornerInfo[];
  marshalSectors: MarshalSectorInfo[];
}
