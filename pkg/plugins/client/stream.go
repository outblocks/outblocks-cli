package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/outblocks/outblocks-cli/pkg/logger"
	plugin_go "github.com/outblocks/outblocks-plugin-go"
)

type SenderStream struct {
	log  logger.Logger
	conn net.Conn
	r    *bufio.Reader
}

func NewSenderStream(log logger.Logger, c net.Conn) *SenderStream {
	return &SenderStream{
		log:  log,
		conn: c,
		r:    bufio.NewReader(c),
	}
}

func (s *SenderStream) Send(req plugin_go.Request) error {
	return writeRequest(s.conn, req)
}

func (s *SenderStream) Recv() (plugin_go.Response, error) {
	for {
		res, err := readResponse(s.r)
		if err != nil {
			return nil, err
		}

		handled, err := s.handleResponse(res)
		if err != nil || !handled {
			return res.Response, err
		}
	}
}

func (s *SenderStream) DrainAndClose() error {
	for {
		_, err := s.Recv()
		if err != nil {
			break
		}
	}

	return s.conn.Close()
}

func (s *SenderStream) Close() error {
	return s.conn.Close()
}

func writeRequest(conn net.Conn, req plugin_go.Request) error {
	_ = conn.SetWriteDeadline(time.Now().Add(DefaultTimeout))

	// Send header.
	data, err := json.Marshal(&plugin_go.RequestHeader{
		Type: req.Type(),
	})
	if err != nil {
		return fmt.Errorf("header encoding error: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	_, err = conn.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	// Send request itself.
	data, err = json.Marshal(req)
	if err != nil {
		return fmt.Errorf("request encoding error: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	_, err = conn.Write([]byte{'\n'})
	if err != nil {
		return fmt.Errorf("plugin comm error: %w", err)
	}

	return nil
}

func readResponse(r *bufio.Reader) (*ResponseWithHeader, error) {
	// Read header.
	data, err := r.ReadBytes('\n')
	if err == io.EOF {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("reading header error: %w", err)
	}

	var header plugin_go.ResponseHeader

	err = json.Unmarshal(data, &header)
	if err != nil {
		return nil, fmt.Errorf("header decode error: %w", err)
	}

	// Read actual response.
	data, err = r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("reading response error: %w", err)
	}

	res := mapResponseType(&header)
	if res == nil {
		return nil, fmt.Errorf("unknown response type: %w", err)
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, fmt.Errorf("response decode error: %w", err)
	}

	return &ResponseWithHeader{
		Header:   header,
		Response: res,
	}, nil
}

func (s *SenderStream) handleResponse(res *ResponseWithHeader) (handled bool, err error) {
	switch r := res.Response.(type) {
	case *plugin_go.PlanResponse,
		*plugin_go.GetStateResponse,
		*plugin_go.SaveStateResponse,
		*plugin_go.EmptyResponse,
		*plugin_go.DataResponse,
		*plugin_go.ValidationErrorResponse,
		*plugin_go.ApplyDoneResponse,
		*plugin_go.ApplyResponse,
		*plugin_go.LockErrorResponse,
		*plugin_go.InitResponse:
		return false, nil

	case *plugin_go.PromptConfirmation:
		confirmed := r.Default

		err = survey.AskOne(&survey.Confirm{
			Message: r.Message,
		}, &confirmed)
		if err != nil {
			return true, err
		}

		return true, s.Send(&plugin_go.PromptConfirmationAnswer{
			Confirmed: confirmed,
		})
	case *plugin_go.PromptInput:
		var input string

		err = survey.AskOne(&survey.Input{
			Message: r.Message,
			Default: r.Default,
		}, &input)
		if err != nil {
			return true, err
		}

		return true, s.Send(&plugin_go.PromptInputAnswer{
			Answer: input,
		})
	case *plugin_go.PromptSelect:
		var input string

		sel := &survey.Select{
			Message: r.Message,
			Options: r.Options,
		}

		if r.Default != "" {
			sel.Default = r.Default
		}

		err = survey.AskOne(sel, &input)
		if err != nil {
			return true, err
		}

		return true, s.Send(&plugin_go.PromptInputAnswer{
			Answer: input,
		})
	case *plugin_go.ErrorResponse:
		return true, fmt.Errorf(r.Error)
	case *plugin_go.MessageResponse:
		switch r.Level() {
		case plugin_go.MessageLogLevelDebug:
			s.log.Debugln(r.Message)
		case plugin_go.MessageLogLevelInfo:
			s.log.Infoln(r.Message)
		case plugin_go.MessageLogLevelWarn:
			s.log.Warnln(r.Message)
		case plugin_go.MessageLogLevelSuccess:
			s.log.Successln(r.Message)
		case plugin_go.MessageLogLevelError:
			s.log.Errorln(r.Message)
		default:
			return true, fmt.Errorf("unknown message level: %s", r.Level())
		}
	case *plugin_go.UnhandledResponse:
	default:
		return true, fmt.Errorf("response not handled! type: %d", res.Response.Type())
	}

	return true, nil
}
