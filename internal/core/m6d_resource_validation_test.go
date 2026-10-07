package core_test

import (
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6dResourceValidation_rejectsInvalidAndForeignWithoutWrites(t *testing.T) {
	f := newM6dHTTPFixture(t)
	if _, err := f.pool.Exec(t.Context(), `INSERT INTO devices(pond_id,device_no,secret_hash,disabled_at) VALUES(8301,'m6d-disabled','hash',now())`); err != nil {
		t.Fatal(err)
	}
	for _, resources := range []domain.APIKeyResourceScope{
		{FarmIDs: []int64{0}},
		{FarmIDs: []int64{-1}},
		{PondIDs: []int64{0}},
		{DeviceNos: []string{""}},
		{DeviceNos: []string{"  "}},
		{FarmIDs: []int64{8303}},
		{PondIDs: []int64{8303}},
		{DeviceNos: []string{"m6d-foreign"}},
		{FarmIDs: []int64{99999}},
		{PondIDs: []int64{99999}},
		{DeviceNos: []string{"missing-device"}},
		{DeviceNos: []string{"m6d-disabled"}},
	} {
		_, secret, err := f.svc.IssueAPIKey(f.owner, 8301, "invalid-scope", []string{"ponds:read"}, resources, 8301)
		if err == nil || secret != "" {
			t.Fatal("invalid or foreign resource scope issued credentials")
		}
	}
	var keys, audit int
	if err := f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM api_keys),(SELECT count(*) FROM audit_events WHERE action LIKE 'api_key.%')`).Scan(&keys, &audit); err != nil {
		t.Fatal(err)
	}
	if keys != 0 || audit != 0 {
		t.Fatal("rejected resources left partial key/audit state")
	}
	f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{FarmIDs: []int64{8301, 8301}, PondIDs: []int64{8301}, DeviceNos: []string{"m6d-allowed"}})
}

func TestM6dRotation_rejectsChangedResourceOwnershipWithoutWrites(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, _ := f.issue(t, []string{"devices:read"}, domain.APIKeyResourceScope{DeviceNos: []string{"m6d-allowed"}})
	if _, err := f.pool.Exec(t.Context(), `UPDATE devices SET pond_id=8303 WHERE device_no='m6d-allowed'`); err != nil {
		t.Fatal(err)
	}
	_, secret, err := f.svc.RotateAPIKey(f.owner, 8301, 8301, key.KeyID)
	if err == nil || secret != "" {
		t.Fatal("rotation issued credentials for a foreign resource")
	}
	var keys, rotations int
	var active bool
	if err := f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM api_keys),(SELECT count(*) FROM audit_events WHERE action='api_key.rotated'),revoked_at IS NULL FROM api_keys WHERE key_id=$1`, key.KeyID).Scan(&keys, &rotations, &active); err != nil {
		t.Fatal(err)
	}
	if keys != 1 || rotations != 0 || !active {
		t.Fatal("failed rotation left partial key/audit state")
	}
}
