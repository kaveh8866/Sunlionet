# نصب و راه‌اندازی (توسعه)

این پروژه دو باینری تولید می‌کند:

- ShadowNet-Inside: برای داخل ایران (با تگ `inside`)
- ShadowNet-Outside: برای خارج از ایران (با تگ `outside`)

## پیش‌نیازها

- Go نسخه 1.21 به بالا

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

