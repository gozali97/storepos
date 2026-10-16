# Varian retail — RTL-001

`POST /api/v1/catalog/variant-combinations` menerima:

```json
{"sku_prefix":"KAOS","attributes":[{"name":"Ukuran","values":["S","M","L"]},{"name":"Warna","values":["Hitam","Putih"]}]}
```

Menghasilkan 6 kandidat `{name,sku,attributes}`. Generator hanya pratinjau, tidak menulis DB. Klien boleh mengedit SKU dan memilih kombinasi, kemudian menambahkan price_cents/barcodes/base_unit/allow_fractional sebelum mengirim sebagai variants di POST /products.

Maksimal 3 nama atribut dan 100 varian per pembuatan produk. Nama atribut/nilai duplikat case-insensitive ditolak; kombinasi atribut duplikat dan SKU duplikat ditolak. SKU tetap unik per tenant di DB, sehingga kandidat generator yang berbenturan harus diedit sebelum simpan.

`PATCH /api/v1/variants/{id}/status`, If-Match, `{status:"inactive"}` menonaktifkan kombinasi. Status active dapat mengaktifkannya lagi; varian archived tidak dapat diaktifkan lewat endpoint ini. Perubahan status teraudit dan versi lama ditolak.

Pencarian katalog mengembalikan attributes hanya untuk kombinasi aktif. Harga/barcode/saldo tiap varian tetap independen lewat struktur yang sudah ada. UI pemilih varian, edit SKU/atribut setelah produk tersimpan, penambahan varian ke produk lama dan snapshot atribut pada struk belum tersedia.
