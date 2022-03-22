module github.com/Financial-Times/notifications-push/v5

go 1.17

require (
	github.com/Financial-Times/go-fthealth v0.0.0-20171204124831-1b007e2b37b7
	github.com/Financial-Times/go-logger/v2 v2.0.1
	github.com/Financial-Times/kafka-client-go v1.0.1
	github.com/Financial-Times/kafka-client-go/v2 v2.0.0
	github.com/Financial-Times/service-status-go v0.0.0-20160323111542-3f5199736a3d
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/gorilla/mux v1.4.1-0.20170704074345-ac112f7d75a0
	github.com/jawher/mow.cli v0.0.0-20170430135212-8327d12beb75
	github.com/pkg/errors v0.8.0
	github.com/samuel/go-zookeeper v0.0.0-20201211165307-7117e9ea2414
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/wvanbergen/kazoo-go v0.0.0-20180202103751-f72d8611297a
)

require (
	github.com/DataDog/zstd v1.3.6-0.20190409195224-796139022798 // indirect
	github.com/Financial-Times/kafka v1.2.0 // indirect
	github.com/Shopify/sarama v1.30.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/frankban/quicktest v1.14.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/go-version v0.0.0-20170202080759-03c5bf6be031 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/stretchr/objx v0.1.0 // indirect
	golang.org/x/crypto v0.0.0-20210920023735-84f357641f63 // indirect
	golang.org/x/net v0.0.0-20210917221730-978cfadd31cf // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.2.3 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/Shopify/sarama => github.com/Shopify/sarama v1.23.0
