# Play Store listing draft (Phase 8)

A starting draft for the human who actually creates the Play Console listing
— none of this has been submitted anywhere; there is no Play Console access
here, and publishing needs explicit human approval and human-held credentials
(see `docs/roadmap.md` Phase 8). Written in Bahasa Indonesia, matching the
app's target market (IDR, Indonesian merchant examples throughout the
codebase) — translate if the actual launch market differs.

## App name

**Cashflow** — or a market-checked alternative if this exact name is taken
in Play Console (not checked here).

## Short description (max 80 characters)

```
Catat pemasukan & pengeluaran — scan struk otomatis pakai AI. Gratis.
```
(69 characters)

## Full description

```
Cashflow membantu kamu mencatat dan memahami arus kas — pemasukan dan
pengeluaran — sendirian atau bersama keluarga/usahamu, dengan cara yang
cepat dan rapi.

FITUR UTAMA
• Catat transaksi manual dalam hitungan detik
• Scan struk belanja — AI otomatis membaca nama toko, tanggal, dan total,
  kamu tinggal periksa dan simpan (tidak pernah tersimpan otomatis tanpa
  persetujuanmu)
• Kelola beberapa akun (Tunai, Rekening Bank, dll) dengan saldo berjalan
• Kategori pemasukan/pengeluaran yang bisa disesuaikan
• Laporan bulanan: ringkasan, rincian per kategori, dan tren 6 bulan terakhir
• Workspace bersama — undang keluarga atau rekan usaha (UMKM) untuk
  mencatat kas yang sama, dengan peran pemilik/anggota
• Gratis, tanpa iklan

PRIVASI
Secara baku data kamu bersifat pribadi. Kamu yang memutuskan kalau mau
berbagi workspace dengan orang lain — dan itu pun cuma untuk workspace yang
kamu pilih bagikan, bukan seluruh datamu. Foto struk hanya diproses sesaat
untuk membaca isinya lalu tidak disimpan di server. Baca kebijakan privasi
lengkap kami di dalam aplikasi.

Cashflow dibuat untuk siapa saja yang ingin lebih paham ke mana uangnya
pergi — sendirian atau bersama, tanpa ribet, tanpa biaya.
```

## Category

Finance

## Content rating

Likely "Everyone" — an expense tracker with no user-generated content shown
to other users, no chat, no ads. Confirm via Play Console's rating
questionnaire when the listing is actually created; not filled out here.

## Screenshots (not produced)

Play Store requires real device/emulator screenshots of the actual running
app — impossible to produce without a Flutter SDK to run it (see
`mobile/README.md`). Take these once the app has been verified running:
suggested shots are the Transactions list, the receipt-scan review screen,
and the Reports screen with the chart.

## App icon

A first-draft icon exists at `mobile/assets/icon/icon.png` (1024×1024,
generated programmatically — a green wallet/card mark with a trend line,
matching the app's Material seed colour). Wired into `pubspec.yaml` via
`flutter_launcher_icons`; run `dart run flutter_launcher_icons` after
`flutter pub get` to generate the actual Android/iOS launcher icon files.
Treat this as a functional placeholder — a human designer's pass before
public launch is reasonable, not required.

## Data safety section

Play Console has its own separate "Data safety" form (what data is
collected, whether it's shared, how it can be deleted) — this is not that
form, but everything it needs is in `docs/privacy-policy.md` and
`docs/adr/0002-ai-receipt-scanning.md`. In short: account info (email, name)
and financial data (accounts/categories/transactions) are collected and
stored, tied to a workspace — private by default (one user, one workspace,
auto-created at signup), visible to other users only if the account's owner
explicitly invites them into a shared workspace (Phase 9); never sold, and
never shared with anyone outside a workspace the user chose to join. Receipt
photos are sent to a third-party AI provider (Google Gemini) for one-time
processing and never stored. In-app self-service account deletion exists
(`DELETE /auth/me`, see `docs/business-rules.md` and the More screen) and
takes effect immediately — deleting a solely-owned workspace with it, while
leaving a shared workspace intact for its other members.
