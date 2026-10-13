package adminapi

import (
	"context"
	"errors"
	"fmt"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func (s *Server) pondLatest(ctx context.Context, pondID int64) (gin.H, int, error) {
	const pageSize = 200
	var best *domain.Reading
	count := 0
	for offset := 0; ; {
		page, err := s.deps.Store.ListDevicesPage(ctx, domain.DeviceListQuery{PondID: pondID, Limit: pageSize, Offset: offset})
		if err != nil {
			return nil, 0, fmt.Errorf("list pond devices: %w", err)
		}
		count += len(page.Items)
		for _, d := range page.Items {
			if s.deps.Telemetry == nil {
				continue
			}
			rd, err := s.deps.Telemetry.Latest(ctx, d.DeviceNo)
			if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidProductModel) {
				continue
			}
			if err != nil {
				return nil, 0, fmt.Errorf("read pond shadow: %w", err)
			}
			if best == nil || rd.Timestamp.After(best.Timestamp) || (rd.Timestamp.Equal(best.Timestamp) && rd.DeviceNo < best.DeviceNo) {
				best = &rd
			}
		}
		if len(page.Items) < pageSize {
			break
		}
		offset += len(page.Items)
	}
	if best == nil {
		return nil, count, nil
	}
	return adminWaterLatestJSON(*best), count, nil
}

func (s *Server) logLatestFailure(ctx context.Context, err error) {
	if s.deps.Logger != nil {
		s.deps.Logger.ErrorContext(ctx, "telemetry latest query failed", "stage", "telemetry_latest", "err", err)
	}
}
