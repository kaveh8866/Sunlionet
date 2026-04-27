# SunLionet Official Distribution Policy

This document defines the official distribution channels for SunLionet and the policy for ensuring user trust and artifact integrity.

## 1. Official Channels

SunLionet is distributed exclusively through open-source and community-vetted channels. We avoid centralized proprietary stores where possible to ensure transparency and resistance to platform-level censorship.

### Primary Source: GitHub Releases
*   **Purpose**: The authoritative source for all signed binaries, checksums, and source code archives.
*   **Who should use it**: Users comfortable with manual downloads or using update managers like Obtainium.
*   **Link**: [https://github.com/kaveh8866/Sunlionet/releases](https://github.com/kaveh8866/Sunlionet/releases)

### Android: F-Droid (Upcoming)
*   **Purpose**: The preferred channel for Android users who prioritize Free and Open Source Software (FOSS).
*   **Who should use it**: Android users who want automated updates through a trusted, metadata-verified repository.

### Android: Direct APK
*   **Purpose**: Direct download for users in regions where app stores are blocked or for those who prefer manual installation.
*   **Who should use it**: Users who cannot access F-Droid or GitHub easily.

## 2. Recommended Install Paths

Our goal is to make installation feel as close to “one click / one tap” as realistically possible, without promising impossible behavior.

In practice, “one click” in this project means:

- one clear primary action that takes you directly to an official source
- minimal confusion and guided next steps
- verification information available before you run/install anything

It does **not** mean silent installs or bypassing platform security prompts. For example: on Android, the website can open the official download/listing and guide you, but Android still requires user confirmation in the installer.

| User Type | Recommended Path | Note |
| :--- | :--- | :--- |
| **Android Users** | **F-Droid** (Recommended) | One tap opens the official listing; installation/updates still follow standard Android confirmation. |
| **Android (Manual)** | **Direct APK** | One tap downloads the official APK; Android will ask you to confirm install (“unknown apps” may be required temporarily). |
| **Linux / Desktop** | **GitHub Releases** | One tap downloads the official bundle; install is a short, explicit step sequence (script + systemd). |
| **Advanced / Recovery** | **Manual / Termux** | For technical diagnostics or custom builds. |

## 3. Trust & Verification

### Why alternative sources?
We use GitHub and F-Droid instead of only centralized stores to ensure:
1.  **Transparency**: Every build can be traced back to the open-source code.
2.  **Censorship Resistance**: Multiple official mirrors reduce dependency on a single gatekeeper.
3.  **No Tracking**: These channels do not require personal accounts or phone numbers to download.
4.  **Resilience**: Direct distribution is harder to block at the platform level.

### Anti-Mirror Warning
Do **NOT** download SunLionet from unofficial mirrors, Telegram groups (unless explicitly verified), or third-party "APK downloader" sites. Modified builds may contain malware or compromise your privacy.

### How to verify
Every official release should publish checksum metadata (for example per-artifact `<file>.sha256`, or a `checksums.txt` bundle) and, when available, signed metadata (`checksums.sig` + `checksums.pub`).

Users are encouraged to verify the hash of their downloaded file before execution:
```bash
sha256sum -c <file>.sha256
```
Detailed instructions are available on the [Verification Guide](/docs/outside/verification).

---

# سیاست توزیع رسمی سان‌لاین‌نت (فارسی)

این سند کانال‌های رسمی توزیع سان‌لاین‌نت و سیاست‌های اعتماد/اصالت فایل‌ها را توضیح می‌دهد.

## ۱) کانال‌های رسمی

سان‌لاین‌نت فقط از طریق کانال‌های متن‌باز و قابل‌بررسی منتشر می‌شود. هدف ما شفافیت، کاهش وابستگی به واسطه‌های متمرکز، و مقاومت بیشتر در برابر سانسور پلتفرم‌هاست.

### منبع اصلی: GitHub Releases
- **برای چه؟** منبع اصلی نسخه‌ها، فایل‌های رسمی، و تاریخچه شفاف انتشار.
- **چه کسانی؟** کاربرانی که دانلود مستقیم را ترجیح می‌دهند یا از ابزارهایی مثل Obtainium استفاده می‌کنند.
- **لینک:** https://github.com/kaveh8866/Sunlionet/releases

### اندروید: F-Droid (به‌زودی)
- **برای چه؟** مسیر پیشنهادی برای کاربران اندروید که به‌روزرسانی‌های خودکار و متن‌باز می‌خواهند.
- **چه کسانی؟** کاربران اندروید که ترجیح می‌دهند از یک مخزن قابل‌تأیید استفاده کنند.

### اندروید: دریافت مستقیم APK
- **برای چه؟** مسیر جایگزین برای مواقعی که فروشگاه‌ها یا دسترسی به گیت‌هاب سخت است.
- **چه کسانی؟** کاربران اندروید که نیاز به نصب دستی دارند.

## ۲) مسیرهای نصب پیشنهادی

هدف ما این است که نصب تا حد امکان «یک‌کلیک / یک‌ضربه و قابل‌فهم» باشد، بدون وعده‌ی غیرواقعی.

در این پروژه، «یک‌کلیک» یعنی:

- یک اقدام اصلی و واضح که شما را مستقیم به منبع رسمی می‌برد
- کمترین سردرگمی و راهنمای گام بعدی
- وجود اطلاعات تأیید اصالت قبل از اجرا/نصب

این مفهوم به معنی «نصب بی‌صدا» یا دور زدن مدل امنیتی پلتفرم‌ها نیست. برای مثال در اندروید، وب‌سایت می‌تواند شما را به مسیر رسمی دانلود/فهرست برنامه هدایت کند، اما خود اندروید همچنان تأیید کاربر را هنگام نصب می‌خواهد.

| نوع کاربر | مسیر پیشنهادی | توضیح |
| :--- | :--- | :--- |
| کاربران اندروید | **F-Droid** (پیشنهادی) | با یک ضربه صفحه رسمی باز می‌شود؛ نصب/به‌روزرسانی طبق تأییدهای استاندارد اندروید انجام می‌شود. |
| اندروید (نصب دستی) | **APK مستقیم** | با یک ضربه APK رسمی دانلود می‌شود؛ اندروید هنگام نصب تأیید می‌خواهد (ممکن است موقتاً اجازه نصب از منبع لازم شود). |
| لینوکس / دسکتاپ | **GitHub Releases** | با یک کلیک فایل رسمی دانلود می‌شود؛ نصب یک توالی کوتاه و شفاف (اسکریپت + systemd) دارد. |
| پیشرفته / بازیابی | **Manual / Termux** | برای کاربران فنی و سناریوهای خاص. |

## ۳) اعتماد و تأیید اصالت

### چرا کانال‌های جایگزین؟
ما از GitHub و F-Droid استفاده می‌کنیم تا:
1) **شفافیت** حفظ شود و نسخه‌ها به کد متن‌باز قابل ربط باشند.  
2) **مقاومت در برابر سانسور** با داشتن چند کانال رسمی افزایش یابد.  
3) **ردیابی کمتر** (بدون نیاز به حساب کاربری/شماره تلفن) ممکن شود.  
4) **تاب‌آوری** افزایش یابد و توزیع سخت‌تر مسدود شود.  

### هشدار درباره منابع غیررسمی
از دانلود سان‌لاین‌نت از منابع غیررسمی، سایت‌های دانلود APK ناشناس، یا فایل‌های «تغییر یافته» خودداری کنید. نسخه‌های دستکاری‌شده می‌توانند امنیت و حریم خصوصی شما را به خطر بیندازند.

### چگونه تأیید کنیم؟
در هر نسخه رسمی باید فایل‌های checksum منتشر شود (مثلاً `<file>.sha256` برای هر فایل یا یک بسته‌ی `checksums.txt`) و در صورت امکان، متادیتای امضا شده (`checksums.sig` + `checksums.pub`).

قبل از اجرای فایل، مقدار SHA256 را بررسی کنید:

```bash
sha256sum -c <file>.sha256
```

راهنمای کامل در [راهنمای تأیید اصالت](/docs/outside/verification) موجود است.
