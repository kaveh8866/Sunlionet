export function ThemeInitScript() {
  const script = `
(function () {
  try {
    var stored = localStorage.getItem("theme");
    var prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
    var theme = stored === "light" || stored === "dark" ? stored : (prefersDark ? "dark" : "light");
    var root = document.documentElement;
    root.classList.toggle("dark", theme === "dark");
    root.classList.toggle("light", theme === "light");
  } catch (e) {}
})();`;

  return <script dangerouslySetInnerHTML={{ __html: script }} />;
}

