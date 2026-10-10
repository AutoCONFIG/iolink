package camera

import (
	"context"
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

type Dependencies struct {
	Store        CameraStore
	License      License
	Cipher       domain.VideoCredentialCipher
	Availability Availability
	LicenseClock LicenseClock
}
type Service struct{ deps Dependencies }

func New(deps Dependencies) *Service { return &Service{deps: deps} }

var _ API = (*Service)(nil)

func (s *Service) run(ctx context.Context, write bool, operation func(CameraTransaction) error) error {
	if s == nil || s.deps.Store == nil {
		return ErrUnavailable
	}
	return s.deps.Store.Run(ctx, func(tx CameraTransaction) error {
		if err := tx.Authorize(ctx, write); err != nil {
			return err
		}
		return operation(tx)
	})
}

func (s *Service) List(ctx context.Context, page Page) (List, error) {
	if page.Limit == 0 {
		page.Limit = 50
	}
	if page.Limit < 1 || page.Limit > 100 || page.AfterID < 0 {
		return List{}, ErrInvalid
	}
	var result List
	err := s.run(ctx, false, func(tx CameraTransaction) error { var err error; result, err = tx.List(ctx, page); return err })
	return result, err
}

func (s *Service) Get(ctx context.Context, id int64) (Camera, error) {
	if id <= 0 {
		return Camera{}, ErrInvalid
	}
	var result Camera
	err := s.run(ctx, false, func(tx CameraTransaction) error { var err error; result, err = tx.Read(ctx, id); return err })
	return result, err
}

func (s *Service) Create(ctx context.Context, cfg Configuration) (Camera, error) {
	return s.configure(ctx, 0, cfg)
}

func (s *Service) Replace(ctx context.Context, id int64, cfg Configuration) (Camera, error) {
	if id <= 0 {
		return Camera{}, ErrInvalid
	}
	return s.configure(ctx, id, cfg)
}

func (s *Service) configure(ctx context.Context, id int64, cfg Configuration) (Camera, error) {
	if cfg.pondID <= 0 || cfg.source == nil {
		return Camera{}, ErrInvalid
	}
	// Recheck authority both before clock observation and in the mutation. The
	// observer uses its own transaction and never borrows a nested pool connection.
	if err := s.run(ctx, true, func(tx CameraTransaction) error { return tx.CheckTarget(ctx, Target{CameraID: id, PondID: cfg.pondID}) }); err != nil {
		return Camera{}, err
	}
	if s.deps.LicenseClock == nil {
		return Camera{}, ErrUnavailable
	}
	if err := s.deps.LicenseClock.ObserveLicenseClock(ctx); err != nil {
		if errors.Is(err, license.ErrClockError) || errors.Is(err, domain.ErrForbidden) {
			return Camera{}, domain.ErrForbidden
		}
		return Camera{}, ErrUnavailable
	}
	var result Camera
	err := s.run(ctx, true, func(tx CameraTransaction) error {
		binding, err := tx.Prepare(ctx, Target{CameraID: id, PondID: cfg.pondID})
		if err != nil {
			return err
		}
		if s.deps.License == nil {
			return ErrUnavailable
		}
		snapshot, err := tx.LicenseSnapshot(ctx)
		if err != nil {
			return err
		}
		if err = s.deps.License.RequireVideo(ctx, snapshot); err != nil {
			if errors.Is(err, domain.ErrForbidden) {
				return domain.ErrForbidden
			}
			return ErrUnavailable
		}
		if s.deps.Availability == nil || s.deps.Cipher == nil {
			return ErrUnavailable
		}
		if err = s.deps.Availability.RequireConfiguration(ctx, cfg.source.Kind()); err != nil {
			return ErrUnavailable
		}
		mutation := Mutation{Binding: binding, Configuration: cfg}
		if len(cfg.credential) > 0 {
			tenantID, _ := domain.TenantID(ctx)
			aad, err := domain.NewVideoCredentialBinding(domain.CameraCredential, tenantID, binding.ID, binding.SourceVersion)
			if err != nil {
				return ErrInternal
			}
			mutation.CredentialCipher, err = s.deps.Cipher.Seal(ctx, aad, cfg.credential)
			if err != nil || len(mutation.CredentialCipher) < 29 {
				return ErrUnavailable
			}
		}

		result, err = tx.Write(ctx, mutation)
		return err
	})
	return result, err
}

func (s *Service) Disable(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalid
	}
	return s.run(ctx, true, func(tx CameraTransaction) error {
		binding, err := tx.Prepare(ctx, Target{CameraID: id})
		if err != nil {
			return err
		}
		return tx.Disable(ctx, binding)
	})
}
