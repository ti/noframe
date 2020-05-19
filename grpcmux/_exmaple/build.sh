mkdir -p "./pkg/swagger/"
mkdir -p "./pkg/go/"
export OUTPUT="./pkg"
docker run --rm  -u $(id -u ${USER}):$(id -g ${USER})  -v $(pwd):$(pwd) -w $(pwd) nanxi/protoc:v1.3.5  --go_out=plugins=grpc:${OUTPUT}/go --govalidators_out=gogoimport=true:${OUTPUT}/go --grpc-gateway_out=logtostderr=false:${OUTPUT}/go  --swagger_out=logtostderr=true:${OUTPUT}/swagger -I. ./*.proto