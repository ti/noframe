module github.com/ti/noframe

require (
	github.com/fsnotify/fsnotify v1.4.8-0.20191012010759-4bf2d1fec783 // indirect
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0
	github.com/grpc-ecosystem/grpc-gateway v1.11.3
	github.com/sirupsen/logrus v1.4.2
	go.etcd.io/etcd v0.0.0-20190917205325-a14579fbfb1a
	google.golang.org/genproto v0.0.0-20190916214212-f660b8655731
	google.golang.org/grpc v1.24.0
	gopkg.in/yaml.v3 v3.0.0-20191120175047-4206685974f2 // indirect
)

replace golang.org/x/crypto v0.0.0-20190911031432-227b76d455e7 => github.com/golang/crypto v0.0.0-20190911031432-227b76d455e7

go 1.13
