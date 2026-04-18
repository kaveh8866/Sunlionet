"use client";

import { useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { uiCopy, type UILang } from "../lib/uiCopy";

type DonationAddress = {
  symbol: string;
  network: string;
  address: string;
};

type DonationQRCardProps = {
  qrValue: string;
  addresses: DonationAddress[];
  lang?: UILang;
};

export function DonationQRCard({ qrValue, addresses, lang = "en" }: DonationQRCardProps) {
  const [copied, setCopied] = useState<string | null>(null);
  const isFa = lang === "fa";
  const copyLabel = uiCopy[lang].buttons.copy;
  const copiedLabel = uiCopy[lang].buttons.copied;

  async function copyValue(value: string, key: string) {
    await navigator.clipboard.writeText(value);
    setCopied(key);
    window.setTimeout(() => setCopied(null), 1600);
  }

  return (
    <section
      aria-labelledby="donation-title"
      className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]"
    >
      <h2 id="donation-title" className="text-2xl font-extrabold tracking-tight text-foreground">
        {isFa ? "حمایت مستقیم و ناشناس" : "Direct Anonymous Donation"}
      </h2>
      <p className="mt-3 text-muted leading-relaxed">
        {isFa
          ? "برای کمک BTC / XMR / USDT را اسکن کنید یا آدرس را کپی کنید. هر کمک به به‌روزرسانی SunLionet در برابر روش‌های جدید DPI کمک می‌کند."
          : "Scan to donate BTC / XMR / USDT or copy the address. One-time or recurring, every contribution helps keep SunLionet updated against new DPI techniques."}
      </p>

      <div className="mt-6 flex justify-center rounded-2xl border border-border bg-white p-4">
        <QRCodeSVG value={qrValue} size={232} level="H" includeMargin />
      </div>

      <p className="mt-4 text-center text-xs text-muted-foreground">
        {isFa ? "کد QR به اولین آدرس لیست اشاره می‌کند (پیشنهادی: XMR)." : "QR points to the first address in the list (recommended: XMR)."}
      </p>

      <div className="mt-6 overflow-x-auto rounded-xl border border-border">
        <table className="w-full min-w-[520px] text-left text-sm">
          <thead className="bg-card text-muted">
            <tr>
              <th className="px-4 py-3 font-semibold">{isFa ? "دارایی" : "Asset"}</th>
              <th className="px-4 py-3 font-semibold">{isFa ? "شبکه" : "Network"}</th>
              <th className="px-4 py-3 font-semibold">{isFa ? "آدرس" : "Address"}</th>
              <th className="px-4 py-3 font-semibold">{isFa ? "عمل" : "Action"}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border bg-card/40 text-muted">
            {addresses.map((item) => (
              <tr key={`${item.symbol}-${item.network}`}>
                <td className="px-4 py-3 font-semibold text-foreground">{item.symbol}</td>
                <td className="px-4 py-3">{item.network}</td>
                <td className="px-4 py-3 font-mono text-xs break-all">{item.address}</td>
                <td className="px-4 py-3">
                  <button
                    type="button"
                    onClick={() => copyValue(item.address, item.symbol)}
                    className="rounded-md border border-border bg-card px-3 py-1.5 text-xs font-semibold text-foreground hover:opacity-90"
                  >
                    {copied === item.symbol ? copiedLabel : copyLabel}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
