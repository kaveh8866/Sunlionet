import { headers } from "next/headers";
import { redirect } from "next/navigation";

export default async function Home() {
  const h = await headers();
  const acceptLanguage = h.get("accept-language") ?? "";
  const prefersFa = /(^|,)\s*fa(?:-|;|,|$)/i.test(acceptLanguage);
  redirect(prefersFa ? "/fa" : "/en");
}
