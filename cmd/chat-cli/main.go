package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"github.com/poponyas/GrpcChat/internal/cli"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", status.Convert(err).Message())
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	command := "chat"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command != "chat" && command != "register" && command != "login" && command != "logout" {
		return fmt.Errorf("unknown command %q; use chat, register, login or logout", command)
	}
	defaultPath, err := cli.DefaultSessionPath()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	addr := flags.String("addr", "", "server address (default 127.0.0.1:50051, or saved session)")
	sessionPath := flags.String("session", defaultPath, "session file; use separate files for different accounts")
	name := flags.String("name", "", "username for register/login")
	fullname := flags.String("fullname", "", "display name for register")
	passwordStdin := flags.Bool("password-stdin", false, "read one password line from stdin instead of a hidden prompt")
	useTLS := flags.Bool("tls", false, "use TLS (required for a remote server)")
	caPath := flags.String("ca", "", "custom PEM CA certificate; implies TLS")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage: chat-cli [chat|register|login|logout] [flags]\n\nregister/login save a session; chat opens the interactive client; logout revokes it.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; use --help")
	}
	var session cli.Session
	if command == "chat" || command == "logout" {
		session, err = cli.LoadSession(*sessionPath)
		if err != nil {
			return err
		}
		if *addr != "" && *addr != session.Server {
			return errors.New("session belongs to another server; log in using --addr and --session")
		}
		*addr = session.Server
		if !*useTLS {
			*useTLS = session.TLS
		}
		if *caPath == "" {
			*caPath = session.CA
		}
	}
	if *addr == "" {
		*addr = "127.0.0.1:50051"
	}
	if *caPath != "" {
		absolute, err := filepath.Abs(*caPath)
		if err != nil {
			return err
		}
		*caPath = absolute
		*useTLS = true
	}
	transport := insecure.NewCredentials()
	if *useTLS {
		config := &tls.Config{MinVersion: tls.VersionTLS12}
		if *caPath != "" {
			data, err := os.ReadFile(*caPath)
			if err != nil {
				return err
			}
			roots, err := x509.SystemCertPool()
			if err != nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(data) {
				return errors.New("CA file contains no certificates")
			}
			config.RootCAs = roots
		}
		transport = credentials.NewTLS(config)
	} else {
		host, _, err := net.SplitHostPort(*addr)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return errors.New("remote connections require --tls or --ca")
		}
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(transport))
	if err != nil {
		return err
	}
	defer conn.Close()
	if command == "chat" {
		return cli.New(conn, session, os.Stdout).Run(ctx, os.Stdin)
	}
	auth := pb.NewAuthServiceClient(conn)
	if command == "logout" {
		call, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if _, err := auth.Logout(session.Context(call), &emptypb.Empty{}); err != nil {
			return err
		}
		if err := os.Remove(*sessionPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println("Сессия завершена.")
		return nil
	}
	if *name == "" {
		return errors.New("--name is required")
	}
	var password string
	if *passwordStdin {
		scanner := bufio.NewScanner(io.LimitReader(os.Stdin, 1024))
		if !scanner.Scan() {
			return errors.New("cannot read password from stdin")
		}
		password = scanner.Text()
	} else {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("use --password-stdin for non-interactive input")
		}
		fmt.Fprint(os.Stderr, "Пароль: ")
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		password = string(data)
	}
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var res *pb.AuthResponse
	if command == "register" {
		res, err = auth.Register(call, &pb.RegisterRequest{Name: *name, Fullname: *fullname, Password: password})
	} else {
		res, err = auth.Login(call, &pb.LoginRequest{Name: *name, Password: password})
	}
	if err != nil {
		return err
	}
	session = cli.Session{Server: *addr, Token: res.Token, UserID: res.Profile.Id, Name: res.Profile.Name, ExpiresAt: res.ExpiresAt.AsTime(), TLS: *useTLS, CA: *caPath}
	if err := cli.SaveSession(*sessionPath, session); err != nil {
		// Do not leave a valid orphan session if saving credentials fails.
		_, _ = auth.Logout(session.Context(call), &emptypb.Empty{})
		return fmt.Errorf("save session: %w", err)
	}
	fmt.Printf("Вы вошли как @%s. Запустите chat-cli --session %s\n", session.Name, *sessionPath)
	return nil
}
