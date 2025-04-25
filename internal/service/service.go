package service

import (
	"context"
)

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
