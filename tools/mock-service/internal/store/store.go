package store

import (
	"context"

	"github.com/raywall/workflows/sample-test/mock-service/internal/domain"
)

type Repository interface {
	List(ctx context.Context) ([]domain.MockDefinition, error)
	Get(ctx context.Context, id string) (domain.MockDefinition, error)
	Save(ctx context.Context, mock domain.MockDefinition) (domain.MockDefinition, error)
	Delete(ctx context.Context, id string) error
}
