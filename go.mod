module github.com/dimmihq/dimmi

go 1.26.0

require (
	blitiri.com.ar/go/spf v1.6.0
	code.dny.dev/ssrf v0.3.0
	codeberg.org/readeck/go-readability/v2 v2.1.3
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2
	github.com/KimMachineGun/automemlimit v1.0.1
	github.com/PuerkitoBio/goquery v1.13.0
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/a-h/templ v0.3.1100
	github.com/abadojack/whatlanggo v1.0.1
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/alexedwards/argon2id v1.0.0
	github.com/aws/aws-sdk-go-v2 v1.47.5
	github.com/aws/aws-sdk-go-v2/config v1.33.13
	github.com/aws/aws-sdk-go-v2/credentials v1.20.13
	github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager v0.4.3
	github.com/aws/aws-sdk-go-v2/service/kms v1.62.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.6
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.79.5
	github.com/aws/smithy-go v1.28.7
	github.com/aymanbagabas/go-udiff v0.4.1
	github.com/bep/imagemeta v1.1.0
	github.com/caddyserver/certmagic v0.25.6
	github.com/cenkalti/backoff/v5 v5.0.3
	github.com/centrifugal/centrifuge v0.40.1
	github.com/charmbracelet/huh v1.0.0
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/coder/websocket v1.8.15
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/danielgtaylor/huma/v2 v2.40.0
	github.com/emersion/go-message v0.18.2
	github.com/emersion/go-msgauth v0.7.0
	github.com/exaring/otelpgx v0.13.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/getsentry/sentry-go v0.50.0
	github.com/go-chi/chi/v5 v5.5.0
	github.com/go-jose/go-jose/v4 v4.1.7
	github.com/go-webauthn/webauthn v0.18.2
	github.com/google/jsonschema-go v0.4.3
	github.com/google/uuid v1.6.0
	github.com/gorilla/feeds v1.2.0
	github.com/gosimple/slug v1.15.0
	github.com/iancoleman/strcase v0.3.0
	github.com/jackc/pgx/v5 v5.12.0
	github.com/jarcoal/httpmock v1.4.2
	github.com/jhillyerd/enmime/v2 v2.5.0
	github.com/jonboulle/clockwork v0.5.0
	github.com/k3a/html2text v1.5.1
	github.com/knadh/koanf/parsers/yaml v1.1.1
	github.com/knadh/koanf/providers/confmap v1.0.1
	github.com/knadh/koanf/providers/env/v2 v2.0.2
	github.com/knadh/koanf/providers/file v1.2.1
	github.com/knadh/koanf/v2 v2.4.1
	github.com/m-mizutani/masq v0.2.3
	github.com/maypok86/otter/v2 v2.4.0
	github.com/medama-io/go-useragent v1.2.5
	github.com/microcosm-cc/bluemonday v1.0.27
	github.com/modelcontextprotocol/go-sdk v1.8.0
	github.com/olivere/vite v0.1.0
	github.com/pgvector/pgvector-go v0.4.1
	github.com/pquerna/otp v1.5.0
	github.com/pressly/goose/v3 v3.29.0
	github.com/prometheus/client_golang v1.25.0
	github.com/realclientip/realclientip-go v1.0.0
	github.com/redis/rueidis v1.0.79
	github.com/redis/rueidis/rueidisaside v1.0.79
	github.com/redis/rueidis/rueidislimiter v1.0.79
	github.com/riverqueue/river v0.51.0
	github.com/riverqueue/river/riverdriver/riverpgxv5 v0.51.0
	github.com/riverqueue/river/rivertype v0.51.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/cors v1.11.1
	github.com/sergeymakinen/go-ico v1.0.0
	github.com/sergi/go-diff v1.4.0
	github.com/sony/gobreaker/v2 v2.4.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/standard-webhooks/standard-webhooks/libraries v0.0.1
	github.com/stretchr/testify v1.12.1
	github.com/temoto/robotstxt v1.1.2
	github.com/tidwall/gjson v1.20.0
	github.com/tink-crypto/tink-go/v2 v2.9.0
	github.com/vearutop/statigz v1.5.0
	github.com/wneessen/go-mail v0.8.1
	github.com/yosida95/uritemplate/v3 v3.0.2
	github.com/yuin/goldmark v1.8.7
	go.abhg.dev/goldmark/frontmatter v0.3.0
	go.abhg.dev/goldmark/toc v0.12.0
	go.jetify.com/typeid v1.3.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.uber.org/zap v1.28.0
	golang.org/x/crypto v0.58.0
	golang.org/x/image v0.47.0
	golang.org/x/net v0.62.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/sync v0.24.0
	golang.org/x/text v0.43.0
	golang.org/x/time v0.16.0
	resty.dev/v3 v3.0.0-rc.4
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/andybalholm/cascadia v1.3.5 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.8 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.6 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/boyter/go-string v1.0.5 // indirect
	github.com/caddyserver/zerossl v0.1.6 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/cention-sany/utf7 v0.0.0-20170124080048-26cad61bd60a // indirect
	github.com/centrifugal/fdelta v0.0.3 // indirect
	github.com/centrifugal/protocol v0.23.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/bubbles v0.21.1-0.20250623103423-23b8fd6302d7 // indirect
	github.com/charmbracelet/bubbletea v1.3.6 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/x/ansi v0.9.3 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13 // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240722160745-212f7b056ed0 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/emersion/go-imap/v2 v2.0.0-beta.8 // indirect
	github.com/emersion/go-sasl v0.0.0-20241020182733-b788ff22d5a6 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.6 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/x v0.3.1 // indirect
	github.com/goccy/go-json v0.11.2 // indirect
	github.com/gofrs/uuid/v5 v5.2.0 // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/inbucket/html2text v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itlightning/dateparse v0.2.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.1 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/libdns/libdns v1.1.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/mholt/acmez/v3 v3.1.7 // indirect
	github.com/miekg/dns v1.1.73 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.3.0 // indirect
	github.com/olekukonko/ll v0.1.8 // indirect
	github.com/olekukonko/tablewriter v1.1.4 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/philhofer/fwd v1.2.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/quagmt/udecimal v1.10.1 // indirect
	github.com/riverqueue/river/riverdriver v0.51.0 // indirect
	github.com/riverqueue/river/rivershared v0.51.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/sergeymakinen/go-bmp v1.0.0 // indirect
	github.com/sethvargo/go-retry v0.5.0 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/zeebo/assert v1.3.0 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261005182115-fad411399dd8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261005182115-fad411399dd8 // indirect
	google.golang.org/grpc v1.84.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
