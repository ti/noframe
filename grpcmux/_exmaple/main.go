package main

import (
	"encoding/json"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/ti/noframe/grpcmux"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"

	"context"
	"fmt"
	pb "github.com/ti/noframe/grpcmux/_exmaple/pkg/go"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	srv := &sayServer{}
	go func() {
		mux := grpcmux.NewServeMux()
		//Handle grpc form /v1/greeter/hello/{id}
		_ = pb.RegisterSayHandlerServer(context.TODO(), mux.ServeMux, srv)
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
		Msg:       fmt.Sprintf("hello %d", req.Id),
		Type:      pb.Type_IMAGES,
		IsSuccess: true,
	}, nil
}

func (h *sayServer) Any(ctx context.Context, in *pb.Data) (*pb.Data, error) {
	var data map[string]interface{}
	err := json.Unmarshal(in.Data, &data)
	if err != nil {
		return nil, status.New(codes.InvalidArgument, "invalid_argument").Err()
	}
	d, _ := json.Marshal(data)

	return &pb.Data{
		Data: d,
	}, nil
}

func (h *sayServer) Errors(ctx context.Context, in *empty.Empty) (*empty.Empty, error) {
	e, _ := status.New(codes.Canceled, "test canceled").WithDetails(&errdetails.ResourceInfo{
		ResourceType: "book",
		ResourceName: "projects/1234/books/5678",
		Owner:        "User",
	},
		&errdetails.RetryInfo{
			RetryDelay: &duration.Duration{Seconds: 60},
		},
		&errdetails.DebugInfo{
			StackEntries: []string{
				"first stack",
				"second stack",
			},
		},
		&errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "name",
					Description: "name is required",
				},
			},
		},
		&errdetails.RequestInfo{
			RequestId:   "12125454",
			ServingData: "yyyy",
		},
		&errdetails.LocalizedMessage{
			Locale:  "zh-cn",
			Message: "中国",
		})

	return nil, e.Err()
}
