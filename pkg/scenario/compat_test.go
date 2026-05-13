package scenario

import "testing"

func TestIsProviderCompatible(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		providers []string
		provider  string
		want      bool
	}{
		{"empty providers matches all", nil, "kind", true},
		{"empty providers matches k3d", nil, "k3d", true},
		{"kind only matches kind", []string{"kind"}, "kind", true},
		{"kind only rejects k3d", []string{"kind"}, "k3d", false},
		{"k3d only matches k3d", []string{"k3d"}, "k3d", true},
		{"k3d only rejects kind", []string{"k3d"}, "kind", false},
		{"both matches kind", []string{"kind", "k3d"}, "kind", true},
		{"both matches k3d", []string{"kind", "k3d"}, "k3d", true},
		{"empty provider string", []string{"kind"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Scenario{Environment: EnvironmentConfig{Providers: tt.providers}}
			if got := s.IsProviderCompatible(tt.provider); got != tt.want {
				t.Errorf("IsProviderCompatible(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestProviderCompatibilityError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		providers []string
		provider  string
		wantNil   bool
	}{
		{"empty providers no error", nil, "kind", true},
		{"compatible no error", []string{"kind"}, "kind", true},
		{"incompatible returns error", []string{"kind"}, "k3d", false},
		{"incompatible error is IncompatibleProviderError", []string{"k3d"}, "kind", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Scenario{
				ID:          "test-scenario",
				Environment: EnvironmentConfig{Providers: tt.providers},
			}
			err := s.ProviderCompatibilityError(tt.provider)
			if tt.wantNil {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			ipe, ok := err.(*IncompatibleProviderError)
			if !ok {
				t.Fatalf("expected *IncompatibleProviderError, got %T", err)
			}
			if ipe.ScenarioID != s.ID {
				t.Errorf("ScenarioID = %q, want %q", ipe.ScenarioID, s.ID)
			}
			if ipe.Running != tt.provider {
				t.Errorf("Running = %q, want %q", ipe.Running, tt.provider)
			}
		})
	}
}
