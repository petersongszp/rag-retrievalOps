module interview-agents

go 1.25.1

require (
	github.com/alicebob/miniredis/v2 v2.37.0
	github.com/apache/thrift v0.21.0
	github.com/bytedance/sonic v1.14.2
	github.com/cloudwego/eino v0.7.28
	github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive v0.0.0-20260129100151-33cdd47ff03a
	github.com/cloudwego/eino-ext/components/embedding/ark v0.1.1
	github.com/cloudwego/eino-ext/components/indexer/milvus v0.0.0-20260129100151-33cdd47ff03a
	github.com/cloudwego/eino-ext/components/model/openai v0.1.8
	github.com/cloudwego/eino-ext/components/retriever/milvus v0.0.0-20260129100151-33cdd47ff03a
	github.com/cloudwego/hertz v0.10.3
	github.com/coze-dev/coze-studio/backend v0.0.0-20251112025517-2945e3dc86e0
	github.com/coze-dev/cozeloop-go v0.1.22
	github.com/coze-dev/cozeloop-go/spec v0.1.4-0.20250829072213-3812ddbfb735
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/larksuite/oapi-sdk-go/v3 v3.4.26
	github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728
	github.com/milvus-io/milvus-sdk-go/v2 v2.4.2
	github.com/panjf2000/ants/v2 v2.11.4
	github.com/plutov/paypal/v4 v4.17.0
	github.com/redis/go-redis/v9 v9.16.0
	github.com/sony/gobreaker v1.0.0
	github.com/stripe/stripe-go/v82 v82.5.1
	golang.org/x/crypto v0.46.0
	golang.org/x/net v0.47.0
	golang.org/x/time v0.14.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/mysql v1.5.7
	gorm.io/driver/sqlite v1.4.3
	gorm.io/gorm v1.25.11
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bluele/gcache v0.0.2 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.4.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/cloudwego/eino-ext/libs/acl/openai v0.1.13 // indirect
	github.com/cloudwego/gopkg v0.1.4 // indirect
	github.com/cloudwego/netpoll v0.7.0 // indirect
	github.com/cockroachdb/errors v1.9.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20211118104740-dabe8e521a4f // indirect
	github.com/cockroachdb/redact v1.1.3 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eino-contrib/jsonschema v1.0.3 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/getsentry/sentry-go v0.34.0 // indirect
	github.com/go-sql-driver/mysql v1.9.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-sqlite3 v1.14.15 // indirect
	github.com/meguminnnnnnnnn/go-openai v0.1.1 // indirect
	github.com/milvus-io/milvus-proto/go-api/v2 v2.5.0-beta.0.20250325034212-6e98baa34971 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nikolalohinski/gonja v1.5.3 // indirect
	github.com/nikolalohinski/gonja/v2 v2.3.1 // indirect
	github.com/nyaruka/phonenumbers v1.6.6 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/volcengine/volc-sdk-golang v1.0.211 // indirect
	github.com/volcengine/volcengine-go-sdk v1.1.44 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yargevad/filepathx v1.0.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/arch v0.23.0 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/apache/thrift => github.com/apache/thrift v0.13.0
