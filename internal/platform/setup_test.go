package platform

import "testing"

func TestSetupInputValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "letter and symbol", password: "Platform-pass1!", wantErr: false},
		{name: "letter only", password: "PlatformPassword", wantErr: true},
		{name: "non-letter only", password: "123456789012", wantErr: true},
		{name: "too short", password: "Abcdef1!", wantErr: true},
		{name: "surrounding whitespace", password: "Platform-pass1! ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := SetupInput{
				PlatformUsername: "platform",
				PlatformPassword: tt.password,
				TenantUsername:   "tenant",
				TenantPassword:   "Tenant-pass2@",
				TenantName:       "Default tenant",
			}
			if gotErr := input.validate(); (gotErr != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestSetupInputValidateRequiresDistinctPasswords(t *testing.T) {
	input := SetupInput{
		PlatformUsername: "platform",
		PlatformPassword: "same-password1!",
		TenantUsername:   "tenant",
		TenantPassword:   "same-password1!",
		TenantName:       "Default tenant",
	}
	if err := input.validate(); err == nil {
		t.Fatal("validate() accepted identical platform and tenant passwords")
	}
}
