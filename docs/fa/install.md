# راهنمای نصب

این مخزن در حال حاضر روی این سناریوها تمرکز دارد:

- **SunLionet Inside**: لینوکس (x86_64 یا arm64) و اندروید (Termux برای توسعه)
- **SunLionet Outside**: هر سیستم‌عامل با اینترنت پایدار (Linux/macOS/Windows)

## برچسب‌های build

- Inside: `-tags inside`
- Outside: `-tags outside`

## مسیر پیشنهادی برای لینوکس (MVP)

برای مسیر کامل MVP روی لینوکس (وارد کردن bundle، انتخاب پروفایل، ساخت کانفیگ، اعتبارسنجی/اجرای sing-box):

- [Linux MVP Install + Run](../install/linux-mvp.md)
- [Linux Smoke Test](../dev/linux-smoke-test.md)

## پیش‌نیازهای توسعه

- Go 1.25+

این مخزن نسخه ابزار Go را در `go.mod` از طریق `toolchain` پین می‌کند. اگر Go شما از toolchain پشتیبانی کند، `go` نسخه پین‌شده را خودکار دانلود می‌کند.

ابزارهای اختیاری:

- `sing-box` (برای اجرای واقعی اتصال)
- یک runtime محلی LLM (مثل llama.cpp) فقط برای build آزمایشی daemon با tag `inside,daemon`

## ساخت (Build)

```bash
mkdir -p bin

go build -tags inside -ldflags="-s -w" -o bin/sunlionet-inside ./cmd/inside/
go build -tags outside -ldflags="-s -w" -o bin/sunlionet-outside ./cmd/outside/
```

## اجرای تست‌ها

- ویندوز:

  ```powershell
  .\scripts\run_tests.ps1
  ```

- لینوکس/مک:

  ```bash
  ./scripts/run_tests.sh
  ```

## بوت‌استرپ اولیه seed برای Inside

Inside از یک API مرکزی دانلود نمی‌کند. seed اولیه از مسیرهای زیر می‌آید:

1. Signal از یک کمک‌کننده مورد اعتماد که Outside را اجرا می‌کند (`snb://v2:` bundle)
2. QR code (حضوری)
3. Bluetooth mesh (از دستگاه‌های Inside نزدیک در زمان blackout)

## نکات اندروید

این مخزن در مسیر release، **APK اندروید** را تولید می‌کند. برای توسعه، می‌توانید Inside را به‌صورت CLI در Termux اجرا کنید یا wrapper اندروید را مستقیم از پوشه `android/` بسازید.

