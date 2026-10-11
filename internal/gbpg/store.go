package gbpg

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/gb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid = errors.New("invalid GB device registration")
	ErrStore   = errors.New("GB device store unavailable")
)

type Store struct {
	pool   *pgxpool.Pool
	cipher domain.VideoCredentialCipher
}

func New(pool *pgxpool.Pool, cipher domain.VideoCredentialCipher) *Store {
	return &Store{pool: pool, cipher: cipher}
}

var _ gb.DeviceStore = (*Store)(nil)

func (s *Store) LookupDevice(ctx context.Context, deviceID string) (gb.RegisteredDevice, error) {
	if s == nil || s.pool == nil || !validDeviceID(deviceID) || s.cipher == nil {
		return gb.RegisteredDevice{}, gb.ErrUnknownDevice
	}
	var id, tenant, version int64
	var realm string
	var enabled bool
	var ciphertext []byte
	var peersJSON string
	err := s.pool.QueryRow(ctx, `SELECT id,tenant_id,credential_version,credential_cipher,realm,enabled,
 COALESCE(array_to_json(allowed_peer_cidrs)::text,'[]')
 FROM video_gb_devices WHERE device_id=$1`, deviceID).
		Scan(&id, &tenant, &version, &ciphertext, &realm, &enabled, &peersJSON)
	if errors.Is(err, pgx.ErrNoRows) || err != nil || !enabled || !validRealm(realm) {
		return gb.RegisteredDevice{}, gb.ErrUnknownDevice
	}
	binding, err := domain.NewVideoCredentialBinding(domain.GBDeviceCredential, tenant, id, version)
	if err != nil {
		return gb.RegisteredDevice{}, gb.ErrUnknownDevice
	}
	password, err := s.cipher.Open(ctx, binding, ciphertext)
	if err != nil || len(password) == 0 || len(password) > 128 || !validText(password) {
		return gb.RegisteredDevice{}, gb.ErrUnknownDevice
	}
	var rawPeers []string
	if err := json.Unmarshal([]byte(peersJSON), &rawPeers); err != nil || len(rawPeers) == 0 {
		return gb.RegisteredDevice{}, gb.ErrUnknownDevice
	}
	peers := make([]netip.Prefix, 0, len(rawPeers))
	for _, raw := range rawPeers {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil || !prefix.IsValid() {
			return gb.RegisteredDevice{}, gb.ErrUnknownDevice
		}
		peers = append(peers, prefix.Masked())
	}
	return gb.RegisteredDevice{DeviceID: deviceID, Realm: realm, Password: password, AllowedPeers: peers}, nil
}

func (s *Store) LookupPeer(ctx context.Context, peer gb.PeerIdentity) (gb.Registration, error) {
	if s == nil || s.pool == nil || !peer.Valid() {
		return gb.Registration{}, gb.ErrPeerNotFound
	}
	var deviceID, transport string
	var address string
	var port int
	var registeredAt, lastSeen, expires time.Time
	var err error
	err = s.pool.QueryRow(ctx, `SELECT d.device_id,d.peer_address::text,d.peer_port,d.transport,
 d.registered_at,d.last_keepalive_at,d.registered_until
 FROM video_gb_devices d
 WHERE d.registration_state='registered' AND d.enabled
   AND d.peer_address=$1::inet AND d.peer_port=$2 AND d.transport=$3
   AND d.registered_until > clock_timestamp()`, peer.Addr.Addr().String(), peer.Addr.Port(), string(peer.Transport)).
		Scan(&deviceID, &address, &port, &transport, &registeredAt, &lastSeen, &expires)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return gb.Registration{}, gb.ErrPeerNotFound
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		prefix, prefixErr := netip.ParsePrefix(address)
		if prefixErr != nil {
			return gb.Registration{}, gb.ErrPeerNotFound
		}
		addr = prefix.Addr()
	}
	identity, err := gb.NewPeerIdentity(netip.AddrPortFrom(addr, uint16(port)), gb.TransportProtocol(transport))
	if err != nil || identity != peer || registeredAt.IsZero() || lastSeen.IsZero() || expires.IsZero() {
		return gb.Registration{}, gb.ErrPeerNotFound
	}
	return gb.Registration{DeviceID: deviceID, Peer: identity, RegisteredAt: registeredAt, LastSeenAt: lastSeen, ExpiresAt: expires, Status: gb.RegistrationOnline}, nil
}

func (s *Store) ApplyRegistration(ctx context.Context, registration gb.Registration) error {
	if s == nil || s.pool == nil || !validDeviceID(registration.DeviceID) || !registration.Peer.Valid() || registration.RegisteredAt.IsZero() {
		return ErrInvalid
	}
	unregister := registration.Status == gb.RegistrationOffline && registration.ExpiresAt.IsZero()
	if !unregister && (registration.Status != gb.RegistrationOnline || registration.ExpiresAt.IsZero() || !registration.ExpiresAt.After(registration.RegisteredAt) || registration.ExpiresAt.Sub(registration.RegisteredAt) > gb.MaxRegistrationTTL || registration.LastSeenAt.IsZero()) {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ErrStore
	}
	defer tx.Rollback(context.Background())
	var id int64
	var enabled bool
	var realm string
	var peersJSON string
	if err = tx.QueryRow(ctx, `SELECT id,enabled,realm,COALESCE(array_to_json(allowed_peer_cidrs)::text,'[]')
 FROM video_gb_devices WHERE device_id=$1 FOR UPDATE`, registration.DeviceID).Scan(&id, &enabled, &realm, &peersJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gb.ErrUnknownDevice
		}
		return ErrStore
	}
	if !enabled || !validRealm(realm) || !allowedPeerJSON(peersJSON, registration.Peer.Addr.Addr()) {
		return gb.ErrPeerNotAllowed
	}
	if unregister {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM video_gb_devices WHERE id=$1 AND registration_state='registered' AND peer_address=$2::inet AND peer_port=$3 AND transport=$4)`, id, registration.Peer.Addr.Addr().String(), registration.Peer.Addr.Port(), string(registration.Peer.Transport)).Scan(&exists)
		if err != nil || !exists {
			return gb.ErrPeerNotFound
		}
		if _, err = tx.Exec(ctx, `UPDATE video_gb_devices SET registration_state='offline',registered_until=NULL,last_keepalive_at=NULL,peer_address=NULL,peer_port=NULL,transport=NULL,registered_at=$2 WHERE id=$1`, id, registration.RegisteredAt); err != nil {
			return ErrStore
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE video_gb_devices SET registration_state='offline',registered_until=NULL,last_keepalive_at=NULL,peer_address=NULL,peer_port=NULL,transport=NULL WHERE id=$1 AND registration_state='registered'`, id); err != nil {
			return ErrStore
		}
		_, err = tx.Exec(ctx, `UPDATE video_gb_devices SET registration_state='registered',registered_at=$2,registered_until=$3,last_keepalive_at=$4,peer_address=$5::inet,peer_port=$6,transport=$7 WHERE id=$1`, id, registration.RegisteredAt, registration.ExpiresAt, registration.LastSeenAt, registration.Peer.Addr.Addr().String(), registration.Peer.Addr.Port(), string(registration.Peer.Transport))
		if err != nil {
			var pgerr *pgconn.PgError
			if errors.As(err, &pgerr) && pgerr.Code == "23505" {
				return gb.ErrPeerConflict
			}
			return ErrStore
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrStore
	}
	return nil
}

func (s *Store) MarkKeepalive(ctx context.Context, deviceID string, peer gb.PeerIdentity, now time.Time, status string) error {
	if s == nil || s.pool == nil || !validDeviceID(deviceID) || !peer.Valid() || now.IsZero() {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ErrStore
	}
	defer tx.Rollback(context.Background())
	if status != "OK" {
		result, execErr := tx.Exec(ctx, `UPDATE video_gb_devices SET registration_state='offline',registered_until=NULL,last_keepalive_at=NULL,peer_address=NULL,peer_port=NULL,transport=NULL WHERE device_id=$1 AND enabled AND registration_state='registered' AND peer_address=$2::inet AND peer_port=$3 AND transport=$4 AND registered_until > clock_timestamp()`, deviceID, peer.Addr.Addr().String(), peer.Addr.Port(), string(peer.Transport))
		if execErr != nil {
			return ErrStore
		}
		if result.RowsAffected() == 0 {
			return gb.ErrPeerNotFound
		}
	} else {
		result, execErr := tx.Exec(ctx, `UPDATE video_gb_devices SET last_keepalive_at=$5 WHERE device_id=$1 AND enabled AND registration_state='registered' AND peer_address=$2::inet AND peer_port=$3 AND transport=$4 AND registered_until > clock_timestamp()`, deviceID, peer.Addr.Addr().String(), peer.Addr.Port(), string(peer.Transport), now)
		if execErr != nil {
			return ErrStore
		}
		if result.RowsAffected() == 0 {
			return gb.ErrPeerNotFound
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrStore
	}
	return nil
}

func validDeviceID(value string) bool {
	if len(value) != 20 {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func validRealm(value string) bool {
	return len(value) > 0 && len(value) <= 128 && validText([]byte(value))
}

func validText(value []byte) bool {
	for _, b := range value {
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

func allowedPeerJSON(raw string, address netip.Addr) bool {
	var peers []string
	if address.IsValid() && json.Unmarshal([]byte(raw), &peers) == nil {
		for _, value := range peers {
			prefix, err := netip.ParsePrefix(value)
			if err == nil && prefix.Contains(address) {
				return true
			}
		}
	}
	return false
}
