package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Novochenko/protos/gen/go/sso"
	"github.com/Novochenko/sso/domain/models"
	"github.com/google/uuid"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	api sso.AuthClient
	log *slog.Logger
}

func New(
	ctx context.Context,
	log *slog.Logger,
	addr string,
	timeout time.Duration,
	retriesCount int,
) (*Client, error) {
	const op = "grpc.New"

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	// Опции для интерсептора grpclog
	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadReceived, grpclog.PayloadSent),
	}

	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &Client{
		api: sso.NewAuthClient(cc),
		log: log,
	}, nil
}

func (c *Client) Find(ctx context.Context, id uuid.UUID) (models.UserAccount, error) {
	const op = "grpc.Find"
	resp, err := c.api.Find(ctx, &sso.FindRequest{
		UserId: id.String(),
	})
	if err != nil {
		return models.UserAccount{}, fmt.Errorf("%s: %w", op, err)
	}

	uuID, err := uuid.Parse(resp.UserAccount.GetUserId())
	if err != nil {
		return models.UserAccount{}, fmt.Errorf("%s: %w", op, err)
	}
	userAcc := models.UserAccount{
		UserId:   uuID,
		UserName: resp.UserAccount.GetUserName(),
	}
	return userAcc, nil
}

func (c *Client) IsAdmin(ctx context.Context, userID string) (bool, error) {
	const op = "grpc.IsAdmin"

	resp, err := c.api.IsAdmin(ctx, &sso.IsAdminRequest{
		UserId: userID,
	})
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return resp.IsAdmin, nil
}

func (c *Client) Login(ctx context.Context, email, password string, appID int64) (string, error) {
	const op = "grpc.Find"
	logRes, err := c.api.Login(ctx, &sso.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    appID,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}
	return logRes.GetToken(), nil
}

func (c *Client) Register(ctx context.Context, email, password string) (string, error) {
	regResp, err := c.api.Register(ctx, &sso.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", nil
	}
	return regResp.GetUserId(), nil
}

func InterceptorLogger(l *slog.Logger) grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
