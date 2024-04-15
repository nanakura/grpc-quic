package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	hello_grpc "gitee.com/asphodelus_dev/grpc-quic/proto"
	"github.com/quic-go/quic-go"
	"google.golang.org/grpc"
	"log"
	"math/big"
	"time"
)

type hello struct {
	hello_grpc.UnimplementedHelloGRPCServer
}

func (s *hello) SayHi(ctx context.Context, req *hello_grpc.Req) (res *hello_grpc.Res, err error) {
	fmt.Println(req.GetMessage())
	return &hello_grpc.Res{Message: "from server"}, nil
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}, nil
}

func main() {
	var (
		client *grpc.ClientConn
		server *grpc.Server

		err error
	)
	tlsConf, err := generateTLSConfig()
	if err != nil {
		panic(err)
	}
	addr := "localhost:1234"
	qln, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		panic(err)
	}
	ln := quic_grpc.Listen(qln)
	server = grpc.NewServer()
	hello_grpc.RegisterHelloGRPCServer(server, &hello{})
	go func() {
		if err := server.Serve(ln); err != nil {
			panic(err)
		}
	}()

	// client
	cliTlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}

	creds := quic_grpc.NewCredentials(cliTlsConf)

	dialer := quic_grpc.NewQuicDialer(cliTlsConf)
	grpcOpts := []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(creds),
	}

	client, err = grpc.Dial(addr, grpcOpts...)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	helloGRPCClient := hello_grpc.NewHelloGRPCClient(client)
	resp, err := helloGRPCClient.SayHi(ctx, &hello_grpc.Req{
		Message: "Tom",
	})
	if err != nil {
		panic(err)
	}
	log.Println(resp.GetMessage())
}
