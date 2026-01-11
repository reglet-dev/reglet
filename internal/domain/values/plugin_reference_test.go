package values

import "testing"

func TestNewPluginReference(t *testing.T) {
	ref := NewPluginReference("reg.io", "org", "repo", "name", "1.0.0")
	if ref.Registry() != "reg.io" {
		t.Errorf("Registry() = %v, want reg.io", ref.Registry())
	}
	if ref.Name() != "name" {
		t.Errorf("Name() = %v, want name", ref.Name())
	}
	if ref.Version() != "1.0.0" {
		t.Errorf("Version() = %v, want 1.0.0", ref.Version())
	}
	if ref.IsEmbedded() {
		t.Error("IsEmbedded should be false")
	}
}

func TestParsePluginReference(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantErr      bool
		wantName     string
		wantVersion  string
		wantRegistry string // Empty if embedded
		wantIsEmbed  bool
	}{
		{
			name:        "Embedded",
			input:       "simple-plugin",
			wantErr:     false,
			wantName:    "simple-plugin",
			wantIsEmbed: true,
		},
		{
			name:        "EmbeddedWithDashes",
			input:       "my-cool-plugin",
			wantErr:     false,
			wantName:    "my-cool-plugin",
			wantIsEmbed: true,
		},
		{
			name:         "FullOCI",
			input:        "ghcr.io/org/repo/plugin:1.0.0",
			wantErr:      false,
			wantName:     "plugin",
			wantVersion:  "1.0.0",
			wantRegistry: "ghcr.io",
			wantIsEmbed:  false,
		},
		{
			name:    "InvalidOCI_NoTag",
			input:   "ghcr.io/org/repo/plugin",
			wantErr: true, // Code splits parts, if no tag, fails?
			// implementation: nameVersion := strings.Split(parts[len(parts)-1], ":"); if len != 2 => error
		},
		{
			name:    "InvalidOCI_TooShort",
			input:   "ghcr.io/plugin:1.0.0",
			wantErr: true, // parts < 4 error in implementation?
			// implementation: parts := strings.Split(ref, "/"); if len(parts) < 4 error "invalid oci reference"
			// Logic assumes registry/org/repo/name:version (4 parts min)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePluginReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePluginReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Name() != tt.wantName {
					t.Errorf("Name() = %v, want %v", got.Name(), tt.wantName)
				}
				if tt.wantIsEmbed {
					if !got.IsEmbedded() {
						t.Error("Should be embedded")
					}
				} else {
					if got.IsEmbedded() {
						t.Error("Should not be embedded")
					}
					if got.Version() != tt.wantVersion {
						t.Errorf("Version() = %v, want %v", got.Version(), tt.wantVersion)
					}
					if got.Registry() != tt.wantRegistry {
						t.Errorf("Registry() = %v, want %v", got.Registry(), tt.wantRegistry)
					}
				}
			}
		})
	}
}

func TestPluginReference_Equals(t *testing.T) {
	r1, _ := ParsePluginReference("ghcr.io/org/repo/name:1.0")
	r2, _ := ParsePluginReference("ghcr.io/org/repo/name:1.0")
	r3, _ := ParsePluginReference("ghcr.io/org/repo/name:2.0")

	if !r1.Equals(r2) {
		t.Error("Identical references should be equal")
	}
	if r1.Equals(r3) {
		t.Error("Different versions should not be equal")
	}
}

func TestPluginReference_String(t *testing.T) {
	// Embedded
	emb, _ := ParsePluginReference("foo")
	if emb.String() != "foo" {
		t.Errorf("Embedded string failed: got %s", emb.String())
	}

	// OCI
	raw := "ghcr.io/org/repo/name:1.2.3"
	oci, _ := ParsePluginReference(raw)
	if oci.String() != raw {
		t.Errorf("OCI string failed: got %s, want %s", oci.String(), raw)
	}
}
