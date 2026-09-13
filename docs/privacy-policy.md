# Kebijakan Privasi Cashflow

**Berlaku sejak:** [ISI TANGGAL SAAT DIPUBLIKASIKAN]
**Terakhir diperbarui:** 13 September 2026

> **Catatan untuk pengembang, hapus sebelum publish:** Dokumen ini adalah
> draf yang mendeskripsikan bagaimana aplikasi *dirancang* untuk menangani
> data, berdasarkan kode yang sudah dibangun (Phases 1–9, lihat
> `docs/roadmap.md`). Sebelum dipublikasikan ke Play Store, dokumen ini
> **harus ditinjau oleh orang yang memahami hukum perlindungan data yang
> berlaku** (mis. UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi di
> Indonesia, dan kebijakan Google Play Data Safety) — ini bukan nasihat hukum.
> Beberapa bagian ditandai `[ISI ...]` dan wajib dilengkapi sebelum publish.

## 1. Siapa kami

Cashflow adalah aplikasi pencatat pemasukan dan pengeluaran, dikembangkan
oleh [ISI NAMA PENGEMBANG / ENTITAS]. Aplikasi ini gratis dan tanpa iklan.
Setiap pengguna memiliki **workspace** pribadinya sendiri secara otomatis
saat mendaftar (cukup untuk penggunaan sendirian, tidak perlu pengaturan
tambahan apa pun); pengguna juga dapat memilih membuat workspace tambahan
dan mengundang orang lain ke dalamnya untuk berbagi pencatatan bersama
(mis. satu keluarga atau usaha kecil/UMKM yang mencatat kas bersama) — lihat
§3 untuk apa artinya ini bagi data Anda.

Jika Anda punya pertanyaan tentang kebijakan ini atau data Anda, hubungi kami
di: **ari.decentindonesia@gmail.com**

## 2. Data yang kami kumpulkan

| Data | Untuk apa | Wajib? |
| --- | --- | --- |
| Email | Membuat akun, login, memulihkan akses, mengirim undangan workspace | Wajib |
| Kata sandi | Disimpan sebagai hash (bcrypt) — kami sendiri tidak pernah bisa melihat kata sandi asli Anda | Wajib |
| Nama tampilan | Ditampilkan di aplikasi (termasuk ke anggota workspace lain yang sama dengan Anda) | Wajib |
| Data akun/kategori/transaksi yang Anda catat | Inti fungsi aplikasi: menghitung saldo dan laporan workspace Anda | Wajib (untuk memakai fitur pencatatan) |
| Foto struk (jika Anda memakai fitur pindai struk AI) | Diproses untuk membaca merchant/tanggal/total secara otomatis | Opsional — Anda bisa selalu mencatat transaksi secara manual |
| Email penerima undangan workspace (jika Anda mengundang seseorang) | Mengirim tautan undangan bergabung ke workspace Anda | Opsional — hanya jika Anda memilih memakai fitur berbagi workspace |

Kami **tidak** mengumpulkan lokasi GPS, kontak, kamera/mikrofon di luar saat
Anda sendiri memilih memotret struk, atau data dari aplikasi lain di
perangkat Anda.

## 3. Bagaimana data Anda digunakan, dan siapa yang bisa melihatnya

- Setiap akun, kategori, dan transaksi dimiliki oleh satu **workspace**, bukan
  langsung oleh satu pengguna. Secara baku, workspace Anda bersifat pribadi
  dan hanya Anda yang menjadi anggotanya — tidak ada pengguna lain yang bisa
  melihat, mengedit, atau bahkan mengetahui data itu ada, kecuali Anda
  sendiri yang memilih membuat workspace bersama dan mengundang mereka.
- **Jika Anda memilih berbagi workspace** (mis. dengan keluarga atau rekan
  usaha): setiap anggota yang Anda undang dan yang menerima undangan itu
  dapat melihat serta menambah/mengubah **seluruh** akun, kategori, dan
  transaksi di workspace bersama tersebut — sama seperti Anda. Nama tampilan
  dan email anggota juga terlihat oleh sesama anggota workspace yang sama
  (untuk menunjukkan siapa saja yang tergabung). Data di workspace pribadi
  Anda yang terpisah tidak ikut terlihat oleh mereka.
- Undangan workspace dikirim melalui tautan sekali pakai lewat email yang
  Anda masukkan; tautan ini kedaluwarsa dalam 7 hari dan hanya bisa dipakai
  satu kali.
- **Fitur pindai struk (opsional):** jika Anda memilih memotret/memindai
  struk, foto tersebut dikirim ke layanan AI pihak ketiga (lihat §4) untuk
  membaca isi struk. Hasilnya berupa **draf** yang harus Anda tinjau dan
  konfirmasi sendiri sebelum tersimpan sebagai transaksi — sistem kami tidak
  pernah menyimpan transaksi secara otomatis tanpa persetujuan Anda.
- **Foto struk itu sendiri tidak kami simpan di server kami.** Foto hanya
  diproses sesaat untuk satu kali panggilan ke layanan AI, lalu dibuang dari
  memori server. Yang kami simpan hanyalah catatan teknis minim (waktu, status
  berhasil/gagal) untuk mencegah penyalahgunaan kuota harian — catatan ini
  **tidak berisi isi struk Anda** (bukan nama toko, bukan nominal, apa pun),
  dan bersifat pribadi per pengguna (tidak dibagikan ke anggota workspace
  lain, berbeda dari data akun/kategori/transaksi di atas).

## 4. Berbagi data dengan pihak ketiga

- **Layanan AI untuk pindai struk**: saat ini kami menggunakan Google Gemini
  untuk memproses foto struk yang Anda kirim melalui fitur pindai. Penyedia
  ini dapat berubah sewaktu-waktu seiring pengembangan aplikasi; kami akan
  memperbarui bagian ini jika terjadi perubahan penyedia. Hanya foto struk
  yang Anda pilih untuk dipindai yang dikirim — bukan data transaksi lain
  Anda.
- **Layanan email**: jika Anda mengundang seseorang ke workspace Anda,
  alamat email yang Anda masukkan dikirimi email undangan melalui penyedia
  layanan email kami.
- Kami **tidak menjual** data Anda kepada siapa pun, untuk tujuan apa pun.
- Kami **tidak membagikan** data Anda untuk keperluan iklan — aplikasi ini
  tidak menampilkan iklan.
- Data dapat diungkapkan jika diwajibkan oleh hukum yang berlaku.

## 5. Penyimpanan dan keamanan

- Kata sandi disimpan sebagai hash satu-arah (bcrypt) — tidak pernah sebagai
  teks biasa.
- Sesi login menggunakan token akses berumur pendek (15 menit) dan token
  refresh yang dapat dicabut; token refresh disimpan di server hanya dalam
  bentuk hash, bukan nilai aslinya.
- Tautan undangan workspace juga disimpan di server hanya dalam bentuk hash,
  bukan tautan aslinya — pola yang sama seperti token refresh di atas.
- Data disimpan di basis data PostgreSQL yang berjalan di server produksi
  kami sendiri.
- Kami menerapkan enkripsi dalam transit (HTTPS, dengan sertifikat yang
  diperbarui otomatis) untuk semua komunikasi antara aplikasi dan server
  kami.

## 6. Hak Anda atas data Anda

- **Melihat dan mengubah**: semua data yang Anda catat (akun, kategori,
  transaksi) dapat Anda lihat, ubah, atau hapus sendiri langsung dari
  aplikasi.
- **Menghapus akun dan seluruh data Anda**: tersedia langsung di dalam
  aplikasi (menu Lainnya → Zona Berbahaya → Hapus Akun Saya, dengan
  konfirmasi mengetik "DELETE") — penghapusan terjadi **seketika**, bukan
  permintaan manual yang diproses nanti. Jika Anda adalah satu-satunya
  anggota sebuah workspace, workspace tersebut beserta seluruh isinya ikut
  terhapus permanen. Jika Anda berbagi workspace dengan orang lain, data
  workspace itu **tetap ada** untuk anggota lain — hanya keanggotaan dan
  akses Anda sendiri yang hilang, sama seperti keluar dari grup bersama;
  ini disengaja, agar penghapusan akun Anda tidak menghapus catatan
  keuangan bersama milik orang lain juga. Anda juga tetap bisa menghubungi
  **ari.decentindonesia@gmail.com** jika butuh bantuan.
- **Portabilitas**: Anda dapat meminta salinan data transaksi Anda melalui
  kontak di atas.

## 7. Berapa lama data disimpan

- Data akun dan transaksi Anda disimpan selama akun/workspace Anda aktif.
- Transaksi yang Anda hapus disimpan sebagai "terhapus" secara internal
  (tidak tampil lagi di aplikasi maupun laporan Anda) untuk menjaga
  konsistensi laporan historis, alih-alih dihapus permanen seketika — namun
  ini tidak mengubah hak Anda pada §6 untuk menghapus akun beserta seluruh
  datanya kapan saja.
- Cadangan (*backup*) basis data disimpan di server kami selama maksimal 30
  hari untuk keperluan pemulihan bencana, lalu dihapus otomatis.
- Catatan teknis pemakaian fitur pindai struk (§3) disimpan untuk keperluan
  pencegahan penyalahgunaan dan dihapus [ISI: kebijakan retensi log, mis.
  "setelah 90 hari" — belum ditentukan].

## 8. Anak-anak

Aplikasi ini tidak ditujukan untuk anak di bawah 13 tahun, dan kami tidak
dengan sengaja mengumpulkan data dari anak di bawah usia tersebut.

## 9. Perubahan kebijakan ini

Kami dapat memperbarui kebijakan ini seiring aplikasi berkembang. Perubahan
akan tercermin pada tanggal "Terakhir diperbarui" di atas.

## 10. Kontak

Pertanyaan, permintaan data, atau permintaan penghapusan akun:
**ari.decentindonesia@gmail.com**
