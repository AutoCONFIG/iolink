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
			}
			if gotErr := input.validate(); (gotErr != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestSetupInputRequiresPlatformUsername(t *testing.T) {
	input := SetupInput{
		PlatformPassword: "same-password1!",
	}
	if err := input.validate(); err == nil {
		t.Fatal("validate() accepted missing platform username")
	}
}
