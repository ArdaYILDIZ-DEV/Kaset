# KASET

<p align="center">
  <strong>vi tarzı tuşlarla çalışan, Türkçe karakterlere duyarlı anlık arama sunan terminal müzik oynatıcı.</strong>
</p>

<p align="center">
  <img alt="Go 1.24+" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white">
  <img alt="Linux" src="https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black">
  <img alt="mpv" src="https://img.shields.io/badge/player-mpv-691F69?logo=mpv&logoColor=white">
  <a href="LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/license-MIT-green.svg"></a>
</p>

KASET, yerel müzik arşivini alt dizinleriyle birlikte tarar ve `mpv` üzerinden çalar. Çalma sırasını uygulamadan çıkmadan düzenleyebilir, karıştırabilir ve çalma listesi olarak kaydedebilirsiniz.

<p align="center">
  <img src="docs/images/kaset-main.png" alt="KASET ana ekranı" width="900">
</p>

## Hızlı başlangıç

Linux'ta [`mpv`](https://mpv.io/) kuruluysa en güncel amd64 dosyasını [GitHub Releases](https://github.com/ArdaYILDIZ-DEV/Kaset/releases) üzerinden indirin ve çalıştırın:

```bash
curl -fL https://github.com/ArdaYILDIZ-DEV/Kaset/releases/latest/download/kaset-linux-amd64 -o kaset && chmod +x kaset
./kaset ~/Music
```

arm64 kullanıyorsanız indirme adresinin sonundaki `amd64` bölümünü `arm64` olarak değiştirin. Yayımlanan dosyaların özetleri [`checksums.txt`](https://github.com/ArdaYILDIZ-DEV/Kaset/releases/latest/download/checksums.txt) içinde bulunur.

## Özellikler

* Alt klasörler dahil dayanıklı müzik arşivi taraması
* Uygulamayı kapatmadan kütüphaneyi yenileme
* Türkçe büyük ve küçük harfleri dikkate alan anlık arama
* Düzenlenebilir ve karıştırılabilir çalma sırası
* Tek parça ve tüm sıra döngüsü
* Kalıcı, eşzamanlı erişime karşı kilitlenen çalma listeleri
* Bozuk veri dosyalarını silmeden yedekleyerek kurtarma
* Oynatılamayan parçayı bildirip sıradaki parçaya geçme
* Son kullanılan kütüphane, ses seviyesi ve görünüm tercihini hatırlama
* Geniş terminallerde yan yana kütüphane ve çalma sırası
* İsteğe bağlı göreli klasör ayrıntıları
* Dar terminal pencerelerine uyum sağlayan Unicode güvenli arayüz
* Uygulama içi klavye yardım ekranı
* Uygulama kapanırken `mpv` sürecini güvenli biçimde sonlandırma

<p align="center">
  <img src="docs/images/kaset-demo.gif" alt="KASET kullanım demosu" width="900">
</p>

## Gereksinimler

* Linux
* [`mpv`](https://mpv.io/)
* ANSI renklerini ve UTF-8 karakterlerini destekleyen bir terminal

## Kaynaktan derleme

Go 1.24 veya üzerini kurduktan sonra depoyu klonlayın ve proje dizininde KASET'i derleyin:

```bash
go build -trimpath -ldflags="-s -w" -o kaset ./cmd/kaset
```

Ardından bir müzik klasörüyle çalıştırın:

```bash
./kaset ~/Music
```

Komutu her dizinden kullanmak için derlenen dosyayı `/usr/local/bin` altına bağlayabilirsiniz:

```bash
sudo ln -sfn "$(pwd)/kaset" /usr/local/bin/kaset
```

## Kullanım

```text
kaset [MÜZİK_KLASÖRÜ]
```

Örnekler:

```bash
kaset ~/Music
kaset .
kaset -h
```

Klasör açıkça verilirse her zaman o klasör kullanılır. Klasör verilmezse son başarıyla kullanılan kütüphane açılır. Kayıtlı klasör bulunamazsa geçerli dizine geri dönülür.

Desteklenen bir ses dosyası bulunamazsa uygulama hata mesajıyla kapanır. Bir alt klasör okunamıyorsa kalan arşiv taranmaya devam eder ve arayüzde uyarı gösterilir.

## Temel kontroller

| Tuş | İşlev |
|-----|-------|
| `j` / `k`, `↑` / `↓` | Listede gezin |
| `Enter` | Seçili parçayı oynat |
| `Space` | Oynatmayı duraklat veya sürdür |
| `n` / `p` | Sonraki veya önceki parçaya geç |
| `/`, `Esc` | Aramayı aç veya kapat |
| `a` / `A` | Seçili parçayı veya görünen tüm parçaları sıraya ekle |
| `Tab` | Kütüphane ve çalma sırası arasında geçiş yap |
| `P` | Çalma listelerini aç veya kapat |
| `?` | Uygulama içi yardımda tüm kontrolleri göster |
| `q` | Uygulamadan çık |

Çalma sırasını yeniden düzenleme, döngü, karıştırma, ses ve görünüm kontrollerinin tamamını görmek için uygulamada `?` tuşuna basın.

Çalan parça sıradan çıkarıldığında kesilmez. Aynı adda bir çalma listesi varsa üzerine yazmadan önce onay istenir; artık kütüphanede bulunmayan yollar yükleme sırasında atlanır. Geçersiz bir çalma listesi dosyası, silinmek yerine `.corrupt-TARİH` uzantısıyla yedeklenir.

## Arayüz davranışı

Terminal en az 100 hücre genişliğindeyse kütüphane solda, çalma sırası veya çalma listeleri sağda gösterilir. `●` işareti ve parlak başlık aktif paneli, `○` işareti ve soluk başlık pasif paneli belirtir. Seçim oku yalnızca aktif panelde görünür; `Tab` odağı kütüphane ve çalma sırası arasında değiştirir. Daha dar terminallerde aynı tuşla değiştirilen tek panel düzeni kullanılır. Dosya ve metadata metinleri terminal hücre genişliğine göre kesilir; geniş karakterler ve emojiler satırı taşırmaz.

Göreli klasör yolları varsayılan olarak gizlidir. Aynı adlı parçaları ayırt etmek istediğinizde `d` ile klasör ayrıntılarını açabilirsiniz; tercih sonraki oturumlarda korunur.

## Veri dosyaları

KASET verileri şu dizinde saklar:

```text
$XDG_CONFIG_HOME/kaset/
```

`XDG_CONFIG_HOME` tanımlı değilse `~/.config/kaset/` kullanılır.

| Dosya | İçerik |
|-------|--------|
| `playlists.json` | Kayıtlı çalma listeleri ve mutlak parça yolları |
| `settings.json` | Son kullanılan kütüphane, ses seviyesi ve görünüm tercihi |
| `*.lock` | Birden fazla süreç arasındaki kısa ömürlü dosya kilitleri |

Yapılandırma dizini `0700`, veri ve kilit dosyaları `0600` izinleriyle oluşturulur. Yazma işlemleri geçici dosyaya yapılıp atomik olarak asıl dosyanın yerine geçirilir.

Örnek ayar verisi:

```json
{
  "version": 1,
  "library": "/home/user/Music",
  "volume": 85,
  "show_folders": false
}
```

## Desteklenen formatlar

```text
mp3  flac  ogg  opus  m4a  aac  wav  wma  ape
```

Bir dosyanın gerçekten oynatılabilmesi sistemdeki `mpv` kurulumuna bağlıdır. KASET yalnızca normal dosyaları kütüphaneye ekler; sembolik bağlantılar tarama sırasında atlanır.

## Proje yapısı

```text
kaset/
├── cmd/kaset/          Komut satırı girişi ve uygulama yaşam döngüsü
├── internal/config/    Kalıcı kullanıcı ayarları
├── internal/library/   Müzik klasörü taraması
├── internal/player/    mpv ve JSON IPC bağlantısı
├── internal/queue/     Çalma sırası
├── internal/playlist/  JSON çalma listesi deposu
└── internal/tui/       Terminal arayüzü
```

Kullanılan temel bileşenler:

* [`Bubble Tea`](https://github.com/charmbracelet/bubbletea): Terminal arayüzü ve olay döngüsü
* [`Bubbles`](https://github.com/charmbracelet/bubbles): Arayüz bileşenleri
* [`Lip Gloss`](https://github.com/charmbracelet/lipgloss): Terminal stilleri
* [`mpv JSON IPC`](https://mpv.io/manual/stable/#json-ipc): Oynatma kontrolü ve durum bilgisi

KASET, `mpv`yi tek bir süreç olarak `--no-config` seçeneğiyle başlatır. IPC komutları `request_id` ile cevaplarına bağlanır. Uygulamanın veya terminalin beklenmedik biçimde kapanması durumunda `mpv`nin arka planda kalmasını önlemek için süreç sonlandırma sinyalleri kullanılır.

## Geliştirme

Tüm doğrulamaları yerelde çalıştırın:

```bash
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -o kaset ./cmd/kaset
```

`mpv` kullanan entegrasyon testleri:

```bash
KASET_INTEGRATION=1 go test \
  -run 'Test(PlayerIntegration|ParentDeathStopsMPV)$' \
  ./internal/player
```

Birim testleri, race detector, statik analiz ve build kontrolü her push ve pull request için GitHub Actions üzerinde çalışır. `v*` etiketi gönderildiğinde release iş akışı Linux amd64 ve arm64 binary'lerini ve SHA-256 özetlerini yayımlar.

## Sorun giderme

### `mpv PATH içinde bulunamadı`

`mpv`yi dağıtımınızın paket yöneticisiyle kurun ve erişilebilir olduğunu doğrulayın:

```bash
mpv --no-config --version
```

### Desteklenen ses dosyası bulunamadı

Doğru klasörü açıkça belirtin:

```bash
kaset /müzik/klasörü
```

### Bazı klasörler taranamadı

KASET erişilemeyen alt klasörleri atlayıp kalan arşivi açar. Uyarı devam ederse ilgili klasörlerin okuma izinlerini kontrol edin.

### Çalma listesi dosyası yedeklendi

Hata mesajındaki `.corrupt-TARİH` dosyası özgün veriyi içerir. Dosyayı düzelttikten sonra `playlists.json` adıyla geri taşıyabilir veya boş çalma listesi deposuyla devam edebilirsiniz.

### Uygulama kapandıktan sonra ses devam ediyor

Çalıştırdığınız komutun güncel çalıştırılabilir dosyayı gösterdiğini kontrol edin:

```bash
readlink -f "$(command -v kaset)"
```

## Katkı

Değişiklik göndermeden önce kapsamı dar tutun, davranış değişikliklerini test edin ve geliştirme kontrollerini çalıştırın. Hata veya özellik önerileri için uygun [issue şablonunu](.github/ISSUE_TEMPLATE) kullanabilirsiniz.

## Lisans

KASET, [MIT Lisansı](LICENSE) altında yayımlanır.
