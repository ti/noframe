FROM nanxi/protoc:go
CMD ["protoc","-I", ".", "--proto_path=/usr/include",  \
  "--go_out","../go","--go_opt","paths=source_relative",  \
  "--go-grpc_out","../go","--go-grpc_opt","paths=source_relative",  \
  "--grpc-gateway_out","../go","--grpc-gateway_opt","logtostderr=true",  \
  "--grpc-gateway_opt","paths=source_relative","--grpc-gateway_opt","generate_unbound_methods=true",  \
  "--validate_out=lang=go,paths=source_relative:../go",  \
  "--openapiv2_out","../openapi","--openapiv2_opt","logtostderr=true", \
  "./*.proto" \
]