"use client";

import { useEffect, useMemo, useState } from "react";

type Theme = "light" | "dark";

function getCurrentTheme(): Theme {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function setTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.classList.toggle("light", theme === "light");
  try {
    localStorage.setItem("theme", theme);
  } catch {}
}

export function ThemeToggle() {
  const [theme, setThemeState] = useState<Theme>("dark");

  useEffect(() => {
    const id = window.setTimeout(() => {
      setThemeState(getCurrentTheme());
    }, 0);
    return () => window.clearTimeout(id);
  }, []);

  const nextTheme = useMemo<Theme>(() => (theme === "dark" ? "light" : "dark"), [theme]);

  return (
    <button
      type="button"
      onClick={() => {
        setTheme(nextTheme);
        setThemeState(nextTheme);
      }}
      className="inline-flex items-center gap-2 rounded-full border border-border bg-card/60 px-3 py-2 text-sm font-semibold text-foreground transition-colors hover:bg-card"
      aria-label={`Switch to ${nextTheme} mode`}
    >
      {theme === "dark" ? (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true" className="text-primary">
          <path
            d="M21 12.8A8.5 8.5 0 0 1 11.2 3a7 7 0 1 0 9.8 9.8Z"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinejoin="round"
          />
        </svg>
      ) : (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true" className="text-primary">
          <path
            d="M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10Z"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinejoin="round"
          />
          <path
            d="M12 1v3M12 20v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M1 12h3M20 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
      )}
      <span className="hidden sm:inline">{theme === "dark" ? "Dark" : "Light"}</span>
    </button>
  );
}
