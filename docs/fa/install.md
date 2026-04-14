# نصب و راه‌اندازی (توسعه)

این پروژه دو باینری تولید می‌کند:

- ShadowNet-Inside: برای داخل ایران (با تگ `inside`)
- ShadowNet-Outside: برای خارج از ایران (با تگ `outside`)

## پیش‌نیازها

- Go نسخه 1.22 به بالا

## بیلد

```bash
mkdir -p bin

go build -tags inside -ldflags="-s -w" -o bin/shadownet-inside ./cmd/inside/
go build -tags outside -ldflags="-s -w" -o bin/shadownet-outside ./cmd/outside/
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

## نکات اندروید

در حال حاضر این مخزن یک APK آمادهٔ تولید ارائه نمی‌دهد. برای توسعه می‌توانید نسخهٔ Inside را در Termux (به‌صورت CLI) اجرا کنید و پروژهٔ «VPN/Foreground Service» را به‌صورت یک پروژهٔ جداگانهٔ اندرویدی نگه دارید.
