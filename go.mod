module github.com/ti/noframe

require (
	github.com/gogo/protobuf v1.2.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/grpc-ecosystem/grpc-gateway v1.6.2
	github.com/sirupsen/logrus v1.2.0
	go.etcd.io/etcd v0.0.0-20181212165745-e57f4f420dfe
	google.golang.org/genproto v0.0.0-20181202183823-bd91e49a0898
	google.golang.org/grpc v1.17.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.2
)

replace (
	golang.org/x/crypto v0.0.0-20180802221240-56440b844dfe => github.com/golang/crypto v0.0.0-20180802221240-56440b844dfe
	golang.org/x/net v0.0.0-20180801234040-f4c29de78a2a => github.com/golang/net v0.0.0-20180801234040-f4c29de78a2a
	golang.org/x/sys v0.0.0-20180903190138-2b024373dcd9 => github.com/golang/sys v0.0.0-20180903190138-2b024373dcd9
	golang.org/x/text v0.3.0 => github.com/golang/text v0.3.0
	golang.org/x/tools v0.0.0-20180828015842-6cd1fcedba52 => github.com/golang/tools v0.0.0-20180828015842-6cd1fcedba52
	google.golang.org/genproto v0.0.0-20181202183823-bd91e49a0898 => github.com/google/go-genproto v0.0.0-20181202183823-bd91e49a0898
	google.golang.org/grpc v1.17.0 => github.com/grpc/grpc-go v1.17.0
	honnef.co/go/tools v0.0.0-20180728063816-88497007e858 => github.com/dominikh/go-tools v0.0.0-20180728063816-88497007e858
)
