package domain

type Registration struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type PlatformUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type PlatformStats struct {
	Tenants       int64 `json:"tenants"`
	Users         int64 `json:"users"`
	ActiveUsers   int64 `json:"active_users"`
	Devices       int64 `json:"devices"`
	OnlineDevices int64 `json:"online_devices"`
}
