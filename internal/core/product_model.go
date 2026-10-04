package core

import (
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
)

func (s *Service) Products() *persistence.ProductStore {
	return persistence.NewProductStoreWithPolicy(s.pool, s.policy)
}
