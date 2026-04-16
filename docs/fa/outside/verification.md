# Outside: تأیید صحت (Verification)

این بخش برای این اهداف طراحی شده است:

- اپراتورها (بازبینی دستی قبل از توزیع)
- CI (شکست دادن pipeline اگر هر بررسی مهم اعتماد شکست بخورد)
- سازگاری با importer در Inside (همان منطق اعتبارسنجی)

## تأیید از روی فایل Bundle

bundleهای رمزگذاری‌شده (پیشنهادی):

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

bundleهای plaintext:

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub
```

فرم کوتاه (پشتیبانی می‌شود):

```bash
go run -tags outside ./cmd/outside --verify ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub
```

## تأیید از روی URI

```bash
go run -tags outside ./cmd/outside verify \
  --uri-file ./dist/bundle.uri.txt \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt
```

## خروجی ماشین‌خوان (CI)

برای CI از خروجی JSON استفاده کنید:

```bash
go run -tags outside ./cmd/outside verify \
  --bundle ./dist/bundle.snb.json \
  --signer-pub ./keys/outside.ed25519.pub \
  --age-identity ./keys/inside.agekey \
  --require-decrypt \
  --json
```

## این تأییدها دقیقاً چه چیزی را بررسی می‌کنند؟

Header:

- پارس strict JSON (فیلد ناشناخته رد می‌شود؛ trailing JSON مجاز نیست)
- `magic`, `bundle_id`, `seq` و timestampها
- اعتبارسنجی expiration و sanity clock-skew
- سازگاری متادیتای cipher/encryption
- متادیتای issuer: مقدار `publisher_key_id` باید با key id امضاکننده مورد اعتماد هم‌خوان باشد
- اعتبار امضا

Payload (برای bundleهای رمزگذاری‌شده نیاز به decrypt دارد):

- پارس strict JSON و نسخه schema
- canonical encoding قطعی (`payload_bytes == canonical(payload)`)
- حضور و سازگاری متادیتای issuer (`notes.issuer_key_id`, `profile.source.publisher_key`)
- قواعد اعتبارسنجی و نرمال‌سازی per-profile
- جلوگیری از تکرارها و مشکلات اعتماد (IDهای تکراری و endpointهای تکراری)
- الزامات template (کلید template هر پروفایل باید وجود داشته باشد؛ متن template باید JSON معتبر باشد)

