"use client";

import { useEffect, useMemo, useState } from "react";

export type DetectedOS =
  | "android"
  | "ios"
  | "windows"
  | "macos"
  | "linux"
  | "raspberrypi"
  | "unknown";

export type DetectedDevice = "mobile" | "desktop" | "unknown";

export type OsDetection = {
  os: DetectedOS;
  device: DetectedDevice;
  label: string;
  confidence: number;
};

function normalize(s: string) {
  return s.toLowerCase();
}

function detectFromNavigator(): OsDetection {
  if (typeof navigator === "undefined") {
    return { os: "unknown", device: "unknown", label: "Unknown", confidence: 0 };
  }

  const ua = normalize(navigator.userAgent || "");
  const platform = normalize((navigator as unknown as { platform?: string }).platform || "");

  const isAndroid = ua.includes("android");
  const isIOS =
    ua.includes("iphone") ||
    ua.includes("ipad") ||
    ua.includes("ipod") ||
    (platform.includes("mac") && (navigator as unknown as { maxTouchPoints?: number }).maxTouchPoints && (navigator as unknown as { maxTouchPoints: number }).maxTouchPoints > 1);
  const isWindows = platform.includes("win") || ua.includes("windows nt");
  const isMac = platform.includes("mac") || ua.includes("mac os x");
  const isLinux = platform.includes("linux") || ua.includes("linux");

  const isMobile = isAndroid || isIOS || ua.includes("mobile");
  const device: DetectedDevice = isMobile ? "mobile" : "desktop";

  if (isAndroid) {
    return { os: "android", device, label: "Android", confidence: 0.95 };
  }
  if (isIOS) {
    return { os: "ios", device, label: "iOS", confidence: 0.9 };
  }
  if (isWindows) {
    return { os: "windows", device, label: "Windows", confidence: 0.9 };
  }
  if (isMac) {
    return { os: "macos", device, label: "macOS", confidence: 0.85 };
  }
  if (isLinux) {
    return { os: "linux", device, label: "Linux", confidence: 0.8 };
  }

  return { os: "unknown", device: "unknown", label: "Unknown", confidence: 0 };
}

export function useOsDetection() {
  const [detection, setDetection] = useState<OsDetection>(() => ({
    os: "unknown",
    device: "unknown",
    label: "Unknown",
    confidence: 0,
  }));

  useEffect(() => {
    setDetection(detectFromNavigator());
  }, []);

  const supportsAutoRecommendation = useMemo(() => detection.os !== "unknown" && detection.confidence >= 0.6, [detection]);

  return { detection, supportsAutoRecommendation };
}

