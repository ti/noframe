all : clean
	mkdir pb
	docker run --rm -v ${PWD}:${PWD} -w ${PWD} nanxi/protoc:lite --go_out=plugins=grpc:pb --grpc-gateway_out=logtostderr=false:pb --rest_out=plugins=rest:pb  -I. *.proto

clean :
	rm -rf pb