package core

import (
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
)

func (s *Service) Products() *persistence.ProductStore { return persistence.NewProductStore(s.pool) }
