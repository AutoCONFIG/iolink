package appapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func (devices fakeDevices) ListPageByUser(ctx context.Context, _ int64, _ domain.DeviceListQuery) (domain.DevicePage, error) {
	items, err := devices.ListByPond(ctx, 1)
	return domain.DevicePage{Items: items, Total: len(items)}, err
}

func (failingDeviceList) ListPageByUser(context.Context, int64, domain.DeviceListQuery) (domain.DevicePage, error) {
	return domain.DevicePage{}, poisonProviderError
}

type failingPondDetail struct{ fakeRepos }

func (failingPondDetail) GetByUser(context.Context, int64, int64) (domain.Pond, error) {
	return domain.Pond{}, poisonProviderError
}

type failingDeviceDetail struct{ fakeDevices }

type missingDevicePage struct{ domain.DeviceRepo }

func (failingDeviceDetail) GetByDeviceNoForUser(context.Context, string, int64) (domain.Device, error) {
	return domain.Device{}, poisonProviderError
}

func TestDeviceListInputRejectsInvalidBounds(t *testing.T) {
	for _, query := range []string{
		"page=%zz",
		"page=", "page=0", "page=-1", "page=invalid", "page=9223372036854775807&page_size=200",
		"page_size=", "page_size=0", "page_size=201", "page_size=invalid",
		"pond_id=", "pond_id=0", "pond_id=-1", "pond_id=invalid",
		"status=", "status=invalid", "name=" + strings.Repeat("a", 129),
	} {
		t.Run(query, func(t *testing.T) {
			// Given: untrusted query values on the actual parsing boundary.
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/api/v1/devices?"+query, nil)
			// When: the query is parsed for the scoped device repository.
			_, err := parseDeviceListInput(c)
			// Then: malformed values never become defaults or overflowing offsets.
			if !errors.Is(err, domain.ErrInvalidRange) {
				t.Fatal("invalid device query accepted")
			}
		})
	}
}

func TestDeviceListInputPreservesValidFilters(t *testing.T) {
	// Given: a valid typed query and a multibyte name at the character limit.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/devices?page=2&page_size=200&pond_id=7&status=disabled&name="+strings.Repeat("塘", 128), nil)
	// When: the complete query is parsed.
	input, err := parseDeviceListInput(c)
	// Then: its page, offset and disabled filter survive without truncation.
	if err != nil || input.page != 2 || input.pageSize != 200 || input.query.Offset != 200 || input.query.PondID != 7 || !input.query.IncludeDisabled || input.query.Status != "disabled" || input.query.Search != strings.Repeat("塘", 128) {
		t.Fatal("valid query was changed or rejected")
	}
}

func TestResourceLookupProviderFailuresUseSafeInternalError(t *testing.T) {
	for _, tc := range []providerHTTPCase{
		{"device_page_capability", "GET", "/devices", func(d *Deps) { d.Devices = missingDevicePage{fakeDevices{}} }},
		{"pond_lookup", "GET", "/ponds/1", func(d *Deps) { d.Ponds = &failingPondDetail{} }},
		{"device_lookup", "GET", "/devices/private-device", func(d *Deps) { d.Devices = failingDeviceDetail{} }},
		{"latest_owner_lookup", "GET", "/water/latest?device_no=private-device", func(d *Deps) { d.Devices = failingDeviceDetail{} }},
		{"history_owner_lookup", "GET", "/water/history?device_no=private-device&metric=ph", func(d *Deps) { d.Devices = failingDeviceDetail{} }},
	} {
		t.Run(tc.name, func(t *testing.T) { exerciseProviderFailure(t, tc) })
	}
}
