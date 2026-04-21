export function generateStaticParams() {
  return [{ lang: "en" }, { lang: "fa" }];
}

export default async function LangLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ lang: string }>;
}) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const isFa = lang === "fa";

  return (
    <div dir={isFa ? "rtl" : "ltr"} className={isFa ? "font-fa" : undefined}>
      {children}
    </div>
  );
}
