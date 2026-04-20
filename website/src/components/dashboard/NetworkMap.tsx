"use client";

import type { KeyboardEvent, MouseEvent } from "react";
import { useMemo, useState } from "react";
import {
  ComposableMap,
  Geographies,
  Geography,
  Marker,
  ZoomableGroup,
} from "react-simple-maps";
import worldTopo from "world-atlas/countries-110m.json";
import type { DashboardSnapshot, RegionAggregate } from "../../lib/dashboard/types";
import { RegionTooltip, type RegionTooltipState } from "./RegionTooltip";
import { statusPalette, helperColor } from "./status";
import { ConnectionFlowLayer } from "./ConnectionFlowLayer";

type GeoProperties = {
  name?: string;
};

type GeoFeature = {
  id?: string | number;
  properties?: GeoProperties;
};

const mapSourceEnv = process.env.NEXT_PUBLIC_SUNLIONET_MAP_URL;
const defaultGeography: unknown = mapSourceEnv && mapSourceEnv.length > 0 ? mapSourceEnv : (worldTopo as unknown);

const centerForView = (view: DashboardSnapshot["view"]): [number, number] => {
  if (view === "iran") return [53.7, 32.0];
  return [10, 20];
};

const zoomForView = (view: DashboardSnapshot["view"]) => {
  if (view === "iran") return 3.4;
  return 1;
};

const buildRegionIndex = (regions: RegionAggregate[]) => {
  const byName = new Map<string, RegionAggregate>();
  for (const r of regions) byName.set(r.countryName.toLowerCase(), r);
  return byName;
};

export const NetworkMap = ({
  snapshot,
  privacyMode,
  showHelpers,
  showFlows,
  onSelectRegion,
}: {
  snapshot: DashboardSnapshot;
  privacyMode: boolean;
  showHelpers: boolean;
  showFlows: boolean;
  onSelectRegion?: (region: RegionAggregate) => void;
}) => {
  const [tooltip, setTooltip] = useState<RegionTooltipState>({ visible: false, x: 0, y: 0 });

  const regionIndex = useMemo(() => buildRegionIndex(snapshot.regions), [snapshot.regions]);

  const privacyThreshold = snapshot.privacyThreshold;

  const getRegion = (geo: GeoFeature): RegionAggregate | undefined => {
    const name = geo?.properties?.name;
    if (!name) return undefined;
    const hit = regionIndex.get(name.toLowerCase());
    if (!hit) return undefined;
    if (privacyMode && hit.sampleCount < privacyThreshold) {
      return { ...hit, status: "unknown" };
    }
    return hit;
  };

  const onRegionEnter = (geo: GeoFeature, evt: MouseEvent<SVGPathElement>) => {
    const region = getRegion(geo);
    if (!region) return;
    setTooltip({ visible: true, x: evt.clientX, y: evt.clientY, region });
  };

  const onRegionMove = (geo: GeoFeature, evt: MouseEvent<SVGPathElement>) => {
    const region = getRegion(geo);
    if (!region) return;
    setTooltip({ visible: true, x: evt.clientX, y: evt.clientY, region });
  };

  const onRegionLeave = () => setTooltip({ visible: false, x: 0, y: 0 });

  const onRegionClick = (geo: GeoFeature) => {
    const region = getRegion(geo);
    if (!region) return;
    onSelectRegion?.(region);
  };

  return (
    <div className="relative rounded-2xl border border-border bg-card/60 shadow-[0_0_0_1px_var(--border)] overflow-hidden">
      <div className="flex items-center justify-between gap-4 px-4 py-3 border-b border-border bg-card">
        <div className="text-sm font-semibold text-foreground">Network Map</div>
        <div className="text-xs text-muted-foreground font-mono">
          {privacyMode ? `privacy≥${privacyThreshold}` : "privacy: off"} • view:{snapshot.view}
        </div>
      </div>
      <div className="p-2 md:p-4">
        <ComposableMap projectionConfig={{ scale: 165 }} style={{ width: "100%", height: "auto" }}>
          <ZoomableGroup center={centerForView(snapshot.view)} zoom={zoomForView(snapshot.view)} minZoom={1} maxZoom={8}>
            <ConnectionFlowLayer flows={snapshot.flows} enabled={showFlows} />
            <Geographies geography={defaultGeography}>
              {({ geographies }: { geographies: unknown[] }) =>
                geographies.map((geo) => {
                  const region = getRegion(geo as unknown as GeoFeature);
                  const status = statusPalette[region?.status ?? "unknown"];
                  return (
                    <Geography
                      key={(geo as unknown as GeoFeature).id ?? (geo as unknown as { rsmKey?: string }).rsmKey ?? Math.random()}
                      geography={geo}
                      onMouseEnter={(evt: MouseEvent<SVGPathElement>) => onRegionEnter(geo as unknown as GeoFeature, evt)}
                      onMouseMove={(evt: MouseEvent<SVGPathElement>) => onRegionMove(geo as unknown as GeoFeature, evt)}
                      onMouseLeave={onRegionLeave}
                      onClick={() => onRegionClick(geo as unknown as GeoFeature)}
                      tabIndex={0}
                      onKeyDown={(evt: KeyboardEvent<SVGPathElement>) => {
                        if (evt.key === "Enter" || evt.key === " ") {
                          evt.preventDefault();
                          onRegionClick(geo as unknown as GeoFeature);
                        }
                      }}
                      style={{
                        default: { fill: status.color, outline: "none", opacity: 0.9 },
                        hover: { fill: status.color, outline: "none", opacity: 1 },
                        pressed: { fill: status.color, outline: "none", opacity: 1 },
                      }}
                    />
                  );
                })
              }
            </Geographies>

            {showHelpers &&
              snapshot.helperNodes.map((n) => (
                <Marker key={n.id} coordinates={n.coords}>
                  <circle r={4.5} fill={helperColor[n.availability]} opacity={0.9} />
                  <circle r={10} fill={helperColor[n.availability]} opacity={0.18} className="motion-safe:animate-ping" />
                </Marker>
              ))}
          </ZoomableGroup>
        </ComposableMap>
      </div>
      <RegionTooltip state={tooltip} />
    </div>
  );
};
