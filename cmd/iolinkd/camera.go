package main

import (
	"errors"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/camerapg"
	"git.hyhy.fun/rsplab/iolink/internal/videocredential"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cameraService(pool *pgxpool.Pool, root string, deps camera.Dependencies) (*camera.Service, error) {
	cipher, err := videocredential.New([]byte(root))
	if err != nil {
		return nil, errors.New("camera credential key unavailable")
	}
	deps.Store = camerapg.New(pool)
	deps.Cipher = cipher
	// Configuration remains unavailable until the approved media runtime is assembled.
	return camera.New(deps), nil
}
