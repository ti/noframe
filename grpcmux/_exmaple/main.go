package main

import (
	"github.com/ti/noframe/grpcmux"
	"net/http"

	"golang.org/x/net/context"
	"fmt"
	"github.com/ti/noframe/grpcmux/_exmaple/pb"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	srv := &sayServer{}
	go func() {
		mux := grpcmux.NewServeMux()
		//Handle grpc form /v1/greeter/hello/{id}
		pb.RegisterSayServerHandlerClient(context.TODO(), mux.ServeMux, srv)
		//Handle common http
		mux.Handle(http.MethodGet, "/v1/home/{id}/users", users)
		log.Println("lis http on  8080")
		//then try http://127.0.0.1:8080/v1/greeter/hello/12
		panic(http.ListenAndServe(":8080", mux))
	}()

	log.Println("lis grpc on 8081")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8081))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	gs := grpc.NewServer()
	pb.RegisterSayServer(gs, srv)
	gs.Serve(lis)
}

func users(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
	w.Write([]byte(pathParams["id"]))
}

type sayServer struct{}

func (h *sayServer) Hello(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	return &pb.Response{
		Msg: fmt.Sprintf("hello %d", req.Id),
	}, nil
}
