package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/empty"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/ti/noframe/grpcmux"
	pb "github.com/ti/noframe/grpcmux/_exmaple/pkg/go"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"net/http"
)

func main() {
	srv := &sayServer{}
	ctx := context.Background()
	go func() {
		mux := grpcmux.NewServeMux()
		// Register generated routes to http mux
		err := pb.RegisterSayHandlerServer(ctx, mux.ServeMux, srv)
		if err != nil {
			panic(err)
		}
		// Register custom route
		mux.Handle(http.MethodGet, "/v1/home/{id}/users",users)
		log.Println("listen http on 8080")
		err = http.ListenAndServe(":8080", mux)
		if err != nil {
			panic(err)
		}
	}()

	log.Println("listen grpc on 8081")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8081))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcOpts := []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_validator.UnaryServerInterceptor(),
		),
		grpc_middleware.WithStreamServerChain(
			grpc_validator.StreamServerInterceptor(),
		),
	}
	gs := grpc.NewServer(grpcOpts...)
	pb.RegisterSayServer(gs, srv)
	gs.Serve(lis)
}

func users(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
	w.Write([]byte(pathParams["id"]))
}

type sayServer struct{
	pb.UnimplementedSayServer
}

func (h *sayServer) Hello(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	err := grpcmux.Validate(ctx, req)
	if err != nil {
		return nil, err
	}
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
