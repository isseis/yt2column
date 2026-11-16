//go:build test

package claudebackend

import (
	"context"
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/llm/claude"
	"github.com/isseis/yt2column/internal/llm/provider"
)

const testKey = "sk-ant-TESTKEY-claudebackend"

func loadConfig(t *testing.T, workspaceID string) config.LLMConfig {
	t.Helper()
	cfg, err := config.LoadExtension(config.ExtensionLookup(testKey, workspaceID))
	if err != nil {
		t.Fatalf("LoadExtension: %v", err)
	}
	return cfg
}

// optionsRecorder returns a backend whose client constructor records the
// options it is given.
func optionsRecorder(got *[]claude.Options) backend {
	return backend{
		newClient: func(o claude.Options) (llm.LLMClient, error) {
			*got = append(*got, o)
			return nil, nil
		},
	}
}

// TestClientsMatchProvider checks the clients are built with the options the
// CLI's provider builds the same stage's client with: the model and effort
// of their own stage, the shared key and workspace ID, and the same
// per-call timeout (internal/llm/provider.LLMTimeout, which this package
// cannot import into a js build and so repeats).
func TestClientsMatchProvider(t *testing.T) {
	for _, workspaceID := range []string{"", "wrkspc_01ABC"} {
		t.Run("workspace="+workspaceID, func(t *testing.T) {
			cfg := loadConfig(t, workspaceID)
			var got []claude.Options
			if _, err := optionsRecorder(&got).NewClients(cfg); err != nil {
				t.Fatalf("NewClients: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("clients built = %d, want 2 (generation, review)", len(got))
			}
			for i, sel := range []config.LLMSelection{cfg.Generation(), cfg.Review()} {
				o := got[i]
				if o.Model != sel.Model() || o.Effort != sel.ClaudeEffort() {
					t.Errorf("client %d: model/effort = %q/%v, want its stage's %q/%v", i, o.Model, o.Effort, sel.Model(), sel.ClaudeEffort())
				}
				if o.Timeout != provider.LLMTimeout {
					t.Errorf("client %d: timeout = %v, want provider.LLMTimeout %v", i, o.Timeout, provider.LLMTimeout)
				}
				if key, err := o.APIKey.Reveal(); err != nil || key != testKey {
					t.Errorf("client %d: key = %q, %v; want the configured one", i, key, err)
				}
				if o.WorkspaceID != cfg.AnthropicWorkspaceID() {
					t.Errorf("client %d: workspace ID differs from the configuration's", i)
				}
			}
		})
	}
}

// TestNewClientsBuildsRealClients checks New's real constructor accepts the
// production profile, so a rejected option would be caught without a network
// call.
func TestNewClientsBuildsRealClients(t *testing.T) {
	deps, err := New().NewClients(loadConfig(t, ""))
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if deps.Generation == nil || deps.Review == nil {
		t.Errorf("deps = %+v, want both clients", deps)
	}
}

// TestNewClientsRejectsOtherProvider checks a configuration whose selection is
// not Claude (the zero configuration here) builds nothing.
func TestNewClientsRejectsOtherProvider(t *testing.T) {
	var got []claude.Options
	_, err := optionsRecorder(&got).NewClients(config.LLMConfig{})
	if !errors.Is(err, ErrNotClaude) {
		t.Errorf("error = %v, want ErrNotClaude", err)
	}
	if len(got) != 0 {
		t.Errorf("%d clients built for a non-Claude selection, want 0", len(got))
	}
}

// TestNewClientsReportsConstructionError checks a client constructor's failure
// is returned and the error wraps its cause.
func TestNewClientsReportsConstructionError(t *testing.T) {
	cause := errors.New("construction failed")
	b := backend{newClient: func(claude.Options) (llm.LLMClient, error) { return nil, cause }}
	if _, err := b.NewClients(loadConfig(t, "")); !errors.Is(err, cause) {
		t.Errorf("error = %v, want it to wrap the constructor's", err)
	}
}

// TestCheckKeyOptions checks the key check gets the configured key and
// workspace ID and the 30-second bound, and returns the call's result.
func TestCheckKeyOptions(t *testing.T) {
	cfg := loadConfig(t, "wrkspc_01ABC")
	sentinel := errors.New("result")
	var got claude.KeyCheckOptions
	b := backend{checkKey: func(_ context.Context, o claude.KeyCheckOptions) error {
		got = o
		return sentinel
	}}
	if err := b.CheckKey(context.Background(), cfg); !errors.Is(err, sentinel) {
		t.Errorf("CheckKey = %v, want the call's result", err)
	}
	if key, err := got.APIKey.Reveal(); err != nil || key != testKey {
		t.Errorf("key = %q, %v; want the configured one", key, err)
	}
	if got.WorkspaceID != cfg.AnthropicWorkspaceID() || got.Timeout != keyCheckTimeout {
		t.Errorf("options = %+v, want the configured workspace ID and %v", got, keyCheckTimeout)
	}
}

// TestInfoDeclaresClaude checks the declaration is the Claude one.
func TestInfoDeclaresClaude(t *testing.T) {
	if New().Info() != extjob.ClaudeInfo() {
		t.Error("Info is not the Claude declaration")
	}
}
