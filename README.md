# KASET

<p align="center">
  <strong>Yerel müzik arşivleri için hızlı ve klavye odaklı terminal müzik oynatıcı.</strong>
</p>

<p align="center">
  <img alt="Go 1.24+" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white">
  <img alt="Linux" src="https://img.shields.io/badge/platform-Linux-FCC624?logo=linux&logoColor=black">
  <img alt="mpv" src="https://img.shields.io/badge/player-mpv-691F69?logo=mpv&logoColor=white">
</p>

KASET, müzik klasörlerini alt dizinleriyle birlikte tarayan terminal tabanlı bir müzik oynatıcıdır. Arama, düzenlenebilir çalma sırası, kalıcı çalma listeleri ve tek parça döngüsü sunar.

<p align="center">
  <img src="docs/images/kaset-main.png" alt="KASET ana ekranı" width="900">
</p>

## Özellikler

* Alt klasörler dahil yerel müzik arşivi taraması
* Anlık arama ve filtrelenmiş sonuçlardan çalma sırası oluşturma
* Parça ekleme, kaldırma ve yeniden sıralama
* Kalıcı çalma listeleri
* Tek parça döngüsü
* Oynat, duraklat, durdur, ileri ve geri sar
* Ses seviyesi ve sessiz mod kontrolü
* Başlık, sanatçı, albüm, süre ve ilerleme bilgisi
* Dar terminal pencerelerine uyum sağlayan arayüz
* Uygulama kapanırken `mpv` sürecini güvenli biçimde sonlandırma

<p align="center">
  <img src="docs/images/kaset-demo.gif" alt="KASET kullanım demosu" width="900">
</p>

## Gereksinimler

* Linux
* Go 1.24 veya üzeri
* [`mpv`](https://mpv.io/)
* ANSI renklerini ve UTF-8 karakterlerini destekleyen bir terminal

## Kurulum

Depoyu klonladıktan sonra proje dizininde binary'yi derleyin:

```bash
go build -trimpath -ldflags="-s -w" -o kaset ./cmd/kaset
```

Ardından bir müzik klasörüyle çalıştırın:

```bash
./kaset ~/Music
```

Komutu her dizinden kullanmak için binary'yi `/usr/local/bin` altına bağlayabilirsiniz:

```bash
sudo ln -sfn "$(pwd)/kaset" /usr/local/bin/kaset
```

```bash
kaset ~/Music
```

Binary aynı konumda kaldığı sürece yeniden derleme sonrasında sembolik bağlantıyı yenilemeniz gerekmez.

## Kullanım

```text
kaset [MÜZİK_KLASÖRÜ]
```

Örnekler:

```bash
# Belirli bir müzik klasörünü tara
kaset ~/Music

# Geçerli dizini tara
kaset .

# Yardımı göster
kaset -h
```

Klasör belirtilmezse geçerli dizin taranır. Desteklenen bir ses dosyası bulunamazsa uygulama hata mesajıyla kapanır.

## Kontroller

### Oynatma

| Tuş       | İşlev                                 |
| --------- | ------------------------------------- |
| `Space`   | Oynat veya duraklat                   |
| `n` / `p` | Sonraki veya önceki parça             |
| `←` / `h` | 5 saniye geri sar                     |
| `→`       | 5 saniye ileri sar                    |
| `l`       | Tek parça döngüsünü aç veya kapat     |
| `+` / `-` | Sesi artır veya azalt                 |
| `m`       | Sessiz modu aç veya kapat             |
| `s`       | Oynatmayı durdur, çalma sırasını koru |
| `q`       | Uygulamadan çık                       |

### Kütüphane

| Tuş                  | İşlev                                               |
| -------------------- | --------------------------------------------------- |
| `j` / `k`, `↑` / `↓` | Listede gezin                                       |
| `g` / `G`            | Listenin başına veya sonuna git                     |
| `Enter`              | Görünen parçaları sıraya al ve seçili parçayı oynat |
| `a`                  | Seçili parçayı sıranın sonuna ekle                  |
| `A`                  | Görünen tüm parçaları sıraya ekle                   |
| `/`                  | Aramayı aç                                          |
| `Esc`                | Aramayı kapat; tekrar basıldığında sorguyu temizle  |
| `t`                  | Alt paneli gizle veya göster                        |
| `Tab`                | Kütüphane ve çalma sırası arasında geçiş yap        |

### Çalma sırası

| Tuş                  | İşlev                                 |
| -------------------- | ------------------------------------- |
| `j` / `k`, `↑` / `↓` | Sırada gezin                          |
| `g` / `G`            | Sıranın başına veya sonuna git        |
| `Enter`              | Seçili parçayı oynat                  |
| `J` / `K`            | Seçili parçayı aşağı veya yukarı taşı |
| `x`                  | Seçili parçayı sıradan çıkar          |
| `c`                  | Sırayı temizle                        |
| `S`                  | Sırayı çalma listesi olarak kaydet    |
| `Tab`                | Kütüphaneye dön                       |

### Çalma listeleri

| Tuş                  | İşlev                                                  |
| -------------------- | ------------------------------------------------------ |
| `P`                  | Çalma listelerini aç veya kapat                        |
| `j` / `k`, `↑` / `↓` | Listede gezin                                          |
| `g` / `G`            | Listenin başına veya sonuna git                        |
| `Enter`              | Seçili listeyi sıraya yükle veya silme işlemini onayla |
| `x`                  | Seçili liste için silme onayını aç                     |
| `Esc`                | Silme işlemini iptal et veya ekranı kapat              |

## Çalma sırası

Kütüphanede `Enter` tuşuna basıldığında ekranda görünen parçalar yeni çalma sırası olur ve seçili parça çalmaya başlar. Arama açıksa yalnızca arama sonuçları kullanılır.

Mevcut sırayı koruyarak parça eklemek için `a`, görünen tüm parçaları eklemek için `A` kullanılabilir. `Tab` ile çalma sırası paneline geçtikten sonra parçalar `J` ve `K` tuşlarıyla yeniden sıralanabilir.

Tek parça döngüsü açıkken çalan parça bittiğinde yeniden başlar. Normal sıraya dönmek için `l` tuşuna tekrar basın.

## Çalma listeleri

Mevcut çalma sırasını kaydetmek için `S` tuşuna basın, bir ad girin ve `Enter` ile onaylayın.

* Aynı adda bir liste varsa üzerine yazmadan önce onay istenir.
* `P` kayıtlı çalma listelerini açar.
* `Enter` seçili listeyi çalma sırasına yükler ancak oynatmayı başlatmaz.
* Çalma sırası panelindeki `Enter`, seçili parçayı başlatır.
* Henüz bir parça çalmıyorsa `Space` sıranın ilk parçasını başlatır.
* `x` seçili liste için silme onayını açar.
* Silinen bir çalma listesi, o anda yüklenmiş çalma sırasını etkilemez.
* Artık kütüphanede bulunmayan dosyalar yükleme sırasında atlanır.

## Veri dosyası

Çalma listeleri şu dosyada saklanır:

```text
$XDG_CONFIG_HOME/kaset/playlists.json
```

`XDG_CONFIG_HOME` tanımlı değilse şu yol kullanılır:

```text
~/.config/kaset/playlists.json
```

Yapılandırma dizini `0700`, veri dosyası ise `0600` izinleriyle oluşturulur. Kayıt işlemi önce geçici bir dosyaya yapılır, ardından bu dosya asıl veri dosyasının yerine geçirilir.

Örnek veri:

```json
{
  "version": 1,
  "playlists": [
    {
      "name": "Gece",
      "tracks": [
        "/home/user/Music/track-one.flac",
        "/home/user/Music/track-two.opus"
      ]
    }
  ]
}
```

Çalma listeleri parçaların tam dosya yollarını saklar. Taşınan veya yeniden adlandırılan dosyalar liste yüklenirken atlanır.

## Desteklenen formatlar

```text
mp3  flac  ogg  opus  m4a  aac  wav  wma  ape
```

Bir dosyanın gerçekten oynatılabilmesi, sistemdeki `mpv` kurulumunun ilgili biçimi desteklemesine bağlıdır.

KASET yalnızca normal dosyaları kütüphaneye ekler. Sembolik bağlantılar tarama sırasında atlanır.

## Proje yapısı

```text
kaset/
├── cmd/kaset/          Komut satırı girişi ve uygulama yaşam döngüsü
├── internal/library/   Müzik klasörü taraması
├── internal/player/    mpv ve JSON IPC bağlantısı
├── internal/queue/     Çalma sırası
├── internal/playlist/  JSON çalma listesi deposu
└── internal/tui/       Terminal arayüzü
```

Kullanılan temel bileşenler:

* [`Bubble Tea`](https://github.com/charmbracelet/bubbletea): Terminal arayüzü
* [`Bubbles`](https://github.com/charmbracelet/bubbles): Arayüz bileşenleri
* [`Lip Gloss`](https://github.com/charmbracelet/lipgloss): Terminal stilleri
* [`mpv JSON IPC`](https://mpv.io/manual/stable/#json-ipc): Oynatma kontrolü ve durum bilgisi

KASET, `mpv`yi tek bir süreç olarak `--no-config` seçeneğiyle başlatır. Uygulamanın veya terminalin beklenmedik biçimde kapanması durumunda `mpv`nin arka planda kalmasını önlemek için süreç sonlandırma sinyalleri kullanılır.

## Geliştirme

Tüm testleri çalıştırmak için:

```bash
go test ./...
```

Veri yarışı kontrolü:

```bash
go test -race ./...
```

Statik analiz:

```bash
go vet ./...
```

`mpv` kullanan entegrasyon testleri:

```bash
KASET_INTEGRATION=1 go test \
  -run 'Test(PlayerIntegration|ParentDeathStopsMPV)$' \
  ./internal/player
```

Release binary oluşturmak için:

```bash
go build -trimpath -ldflags="-s -w" -o kaset ./cmd/kaset
```

## Sorun giderme

### `mpv PATH içinde bulunamadı`

`mpv`yi dağıtımınızın paket yöneticisiyle kurun ve erişilebilir olduğunu doğrulayın:

```bash
mpv --version
```

### Desteklenen ses dosyası bulunamadı

Doğru klasörü verdiğinizden ve dosya uzantısının desteklendiğinden emin olun:

```bash
kaset /müzik/klasörü
```

### Çalma listesinde bazı parçalar eksik

Çalma listeleri parçaların tam dosya yollarını saklar. Dosya taşınmış veya yeniden adlandırılmışsa eski yol geçersiz olur ve KASET parçayı atlar.

### Uygulama kapandıktan sonra ses devam ediyor

Çalıştırılan `kaset` komutunun güncel binary'yi gösterdiğini kontrol edin:

```bash
readlink -f /usr/local/bin/kaset
```

## Katkı

Hata bildirimleri, özellik önerileri ve kod katkıları kabul edilir.

Kod değişikliği göndermeden önce:

1. Depoyu forklayın ve ayrı bir dal oluşturun.
2. Değişikliği mümkün olduğunca dar kapsamlı tutun.
3. Davranış değişiyorsa ilgili testleri ekleyin veya güncelleyin.
4. `go test ./...` ve `go vet ./...` komutlarını çalıştırın.
5. Değişikliği ve nasıl doğrulandığını açıklayan bir pull request açın.

Hata veya özellik önerileri için uygun [issue şablonunu](.github/ISSUE_TEMPLATE) kullanabilirsiniz.
