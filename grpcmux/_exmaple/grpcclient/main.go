package main

import (
	"context"
	"fmt"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	pb "github.com/ti/noframe/grpcmux/_exmaple/pkg/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/resolver"
	"time"
)

func main() {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	target := "127.0.0.1:8081"
	resolver.SetDefaultScheme("dns")
	conn, err := grpc.DialContext(ctx, target, defaultDialOptions(grpc.WithInsecure())...)
	if err != nil {
		err = fmt.Errorf("can not dail client %s for connect: %v", target, err)
		panic(err)
	}
	cli := pb.NewSayClient(conn)
	ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
	resp, err := cli.Hello(ctx, &pb.Request{
		Id:    33,
	})
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(resp.Msg)
	}
}

// defaultDialOptions 通用GRPC 客户端选项
func defaultDialOptions(opts ...grpc.DialOption) []grpc.DialOption {
	tracer := opentracing.GlobalTracer()
	grpcOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(tracer)),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(tracer)),
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": "%s"}`, roundrobin.Name)),
		grpc.WithBlock(),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay: 2 * time.Second,
				MaxDelay:  120 * time.Second,
			},
			MinConnectTimeout: 2 * time.Second,
		}),
	}
	grpcOpts = append(grpcOpts, opts...)
	return grpcOpts
}
