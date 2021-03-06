package server

import (
	"context"
	"errors"
	"net"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/ansel1/merry/v2"
	"github.com/outblocks/outblocks-cli/internal/util"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	apiv1 "github.com/outblocks/outblocks-plugin-go/gen/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	srv     *grpc.Server
	secrets map[string]interface{}
	log     logger.Logger

	addr net.Addr
}

func NewServer(log logger.Logger, secrets map[string]interface{}) *Server {
	return &Server{
		log:     log,
		secrets: secrets,
	}
}

func (s *Server) Addr() net.Addr {
	return s.addr
}

func (s *Server) Serve() error {
	l, err := net.Listen("tcp4", "")
	if err != nil {
		return err
	}

	s.srv = grpc.NewServer()
	apiv1.RegisterHostServiceServer(s.srv, s)

	s.addr = l.Addr()

	go func() {
		_ = s.srv.Serve(l)
	}()

	return nil
}

func (s *Server) Stop() {
	if s.srv != nil {
		s.srv.GracefulStop()
	}
}

func mapPromptError(err error) error {
	if errors.Is(err, terminal.InterruptErr) {
		return status.New(codes.Aborted, err.Error()).Err()
	}

	return err
}

func (s *Server) PromptConfirmation(ctx context.Context, r *apiv1.PromptConfirmationRequest) (*apiv1.PromptConfirmationResponse, error) {
	if !util.IsTerminal() {
		return nil, status.New(codes.Aborted, terminal.InterruptErr.Error()).Err()
	}

	confirmed := r.Default

	err := survey.AskOne(&survey.Confirm{
		Message: r.Message,
	}, &confirmed)
	if err != nil {
		return nil, mapPromptError(err)
	}

	return &apiv1.PromptConfirmationResponse{
		Confirmed: confirmed,
	}, nil
}

func (s *Server) PromptInput(ctx context.Context, r *apiv1.PromptInputRequest) (*apiv1.PromptInputResponse, error) {
	if !util.IsTerminal() {
		return nil, status.New(codes.Aborted, terminal.InterruptErr.Error()).Err()
	}

	var input string

	err := survey.AskOne(&survey.Input{
		Message: r.Message,
		Default: r.Default,
	}, &input)
	if err != nil {
		return nil, mapPromptError(err)
	}

	return &apiv1.PromptInputResponse{
		Answer: input,
	}, nil
}

func (s *Server) PromptSelect(ctx context.Context, r *apiv1.PromptSelectRequest) (*apiv1.PromptSelectResponse, error) {
	if !util.IsTerminal() {
		return nil, status.New(codes.Aborted, terminal.InterruptErr.Error()).Err()
	}

	var input string

	sel := &survey.Select{
		Message: r.Message,
		Options: r.Options,
	}

	if r.Default != "" {
		sel.Default = r.Default
	}

	err := survey.AskOne(sel, &input)
	if err != nil {
		return nil, mapPromptError(err)
	}

	return &apiv1.PromptSelectResponse{
		Answer: input,
	}, err
}

func (s *Server) Log(ctx context.Context, r *apiv1.LogRequest) (*apiv1.LogResponse, error) {
	switch r.Level {
	case apiv1.LogRequest_LEVEL_ERROR:
		s.log.Errorf(r.Message)
	case apiv1.LogRequest_LEVEL_WARN:
		s.log.Warn(r.Message)
	case apiv1.LogRequest_LEVEL_INFO:
		s.log.Info(r.Message)
	case apiv1.LogRequest_LEVEL_DEBUG:
		s.log.Debug(r.Message)
	case apiv1.LogRequest_LEVEL_SUCCESS:
		s.log.Success(r.Message)
	case apiv1.LogRequest_LEVEL_PRINT:
		s.log.Print(r.Message)
	case apiv1.LogRequest_LEVEL_UNSPECIFIED:
		return nil, merry.New("unknown level")
	default:
		s.log.Info(r.Message)
	}

	return &apiv1.LogResponse{}, nil
}

func (s *Server) HostGetSecret(ctx context.Context, r *apiv1.HostGetSecretRequest) (*apiv1.HostGetSecretResponse, error) {
	if s.secrets == nil {
		return &apiv1.HostGetSecretResponse{Specified: false}, nil
	}

	v, ok := s.secrets[r.Key]

	var vstr string

	if ok {
		vstr = v.(string)
	}

	return &apiv1.HostGetSecretResponse{
		Value:     vstr,
		Specified: ok,
	}, nil
}
